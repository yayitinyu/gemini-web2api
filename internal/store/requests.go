package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Store) RecordRequest(ctx context.Context, record RequestRecord) error {
	if record.CreatedAt == 0 {
		record.CreatedAt = unixNow()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO requests(request_id, created_at, endpoint, model, upstream_model, status_code,
 latency_ms, ttfb_ms, input_tokens, output_tokens, stream, account_id, account_label,
 proxy_id, proxy_label, error_code, error_message)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.RequestID,
		record.CreatedAt, record.Endpoint, record.Model, record.UpstreamModel, record.StatusCode,
		record.LatencyMS, record.TTFBMS, record.InputTokens, record.OutputTokens, boolInt(record.Stream),
		record.AccountID, record.AccountLabel, record.ProxyID, record.ProxyLabel,
		truncate(record.ErrorCode, 80), truncate(record.ErrorMessage, 300))
	return err
}

func (s *Store) Requests(ctx context.Context, filter RequestFilter) (RequestPage, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	where, args := requestWhere(filter)
	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM requests `+where, args...).Scan(&total); err != nil {
		return RequestPage{}, err
	}
	query := `
SELECT id, request_id, created_at, endpoint, model, upstream_model, status_code,
       latency_ms, ttfb_ms, input_tokens, output_tokens, stream, account_id,
       account_label, proxy_id, proxy_label, error_code, error_message
FROM requests ` + where + ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, query, append(args, filter.Limit, filter.Offset)...)
	if err != nil {
		return RequestPage{}, err
	}
	defer rows.Close()
	items := make([]RequestRow, 0)
	for rows.Next() {
		var item RequestRow
		var stream int
		if err := rows.Scan(&item.ID, &item.RequestID, &item.CreatedAt, &item.Endpoint, &item.Model,
			&item.UpstreamModel, &item.StatusCode, &item.LatencyMS, &item.TTFBMS,
			&item.InputTokens, &item.OutputTokens, &stream, &item.AccountID,
			&item.AccountLabel, &item.ProxyID, &item.ProxyLabel, &item.ErrorCode,
			&item.ErrorMessage); err != nil {
			return RequestPage{}, err
		}
		item.Stream = stream != 0
		items = append(items, item)
	}
	return RequestPage{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, rows.Err()
}

func requestWhere(filter RequestFilter) (string, []any) {
	parts := []string{"1 = 1"}
	args := make([]any, 0, 2)
	if filter.Model != "" {
		parts = append(parts, "model = ?")
		args = append(args, filter.Model)
	}
	switch filter.Status {
	case "success":
		parts = append(parts, "status_code >= 200 AND status_code < 300")
	case "error":
		parts = append(parts, "(status_code < 200 OR status_code >= 300)")
	}
	return "WHERE " + strings.Join(parts, " AND "), args
}

func (s *Store) Overview(ctx context.Context, since time.Time) (OverviewStats, error) {
	var stats OverviewStats
	var successes int64
	if err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(output_tokens), 0)
FROM requests WHERE created_at >= ?`, since.Unix()).Scan(&stats.Requests, &successes, &stats.OutputTokens); err != nil {
		return stats, err
	}
	if stats.Requests > 0 {
		rate := float64(successes) / float64(stats.Requests) * 100
		stats.SuccessRate = &rate
		rows, err := s.db.QueryContext(ctx, `
SELECT latency_ms FROM requests WHERE created_at >= ? AND status_code >= 200 AND status_code < 300
ORDER BY latency_ms`, since.Unix())
		if err != nil {
			return stats, err
		}
		latencies := make([]int64, 0)
		for rows.Next() {
			var value int64
			if err := rows.Scan(&value); err != nil {
				rows.Close()
				return stats, err
			}
			latencies = append(latencies, value)
		}
		rows.Close()
		if len(latencies) > 0 {
			value := latencies[(len(latencies)-1)/2]
			stats.P50LatencyMS = &value
		}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN enabled = 1 AND status = 'healthy' THEN 1 ELSE 0 END), 0) FROM accounts`).Scan(&stats.Accounts, &stats.Healthy); err != nil {
		return stats, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM proxies WHERE enabled = 1`).Scan(&stats.Proxies); err != nil {
		return stats, err
	}
	return stats, nil
}

func (s *Store) TimeSeries(ctx context.Context, since time.Time, bucket time.Duration) ([]TimePoint, error) {
	if bucket < time.Hour {
		bucket = time.Hour
	}
	bucketSeconds := int64(bucket.Seconds())
	rows, err := s.db.QueryContext(ctx, `
SELECT (created_at / ?) * ? AS bucket,
       COUNT(*),
       SUM(CASE WHEN status_code < 200 OR status_code >= 300 THEN 1 ELSE 0 END),
       CAST(AVG(CASE WHEN status_code >= 200 AND status_code < 300 THEN latency_ms END) AS INTEGER)
FROM requests WHERE created_at >= ? GROUP BY bucket ORDER BY bucket`, bucketSeconds, bucketSeconds, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := make([]TimePoint, 0)
	for rows.Next() {
		var point TimePoint
		var latency sql.NullInt64
		if err := rows.Scan(&point.Bucket, &point.Requests, &point.Failures, &latency); err != nil {
			return nil, err
		}
		if latency.Valid {
			point.LatencyMS = latency.Int64
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func (s *Store) PurgeExpired(ctx context.Context, retentionDays int) error {
	if retentionDays < 1 {
		return fmt.Errorf("retentionDays must be positive")
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	_, err := s.db.ExecContext(ctx, `DELETE FROM requests WHERE created_at < ?`, cutoff)
	return err
}

func percentile(values []int64, p float64) int64 {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	index := int(float64(len(values)-1) * p)
	return values[index]
}
