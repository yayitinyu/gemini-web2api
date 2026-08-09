package store

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"time"
)

const proxyCircuitThreshold = 5

func (s *Store) CreateProxy(ctx context.Context, input ProxyInput) (Proxy, error) {
	input.Label = strings.TrimSpace(input.Label)
	input.URL = strings.TrimSpace(input.URL)
	if input.Label == "" {
		return Proxy{}, errors.New("proxy label is required")
	}
	if err := validateProxyURL(input.URL); err != nil {
		return Proxy{}, err
	}
	sealed, err := s.cipher.Encrypt(input.URL)
	if err != nil {
		return Proxy{}, err
	}
	now := unixNow()
	result, err := s.db.ExecContext(ctx, `
INSERT INTO proxies(label, url_cipher, enabled, status, created_at, updated_at)
VALUES(?, ?, ?, 'unknown', ?, ?)`, input.Label, sealed, boolInt(input.Enabled), now, now)
	if err != nil {
		return Proxy{}, err
	}
	id, _ := result.LastInsertId()
	return s.Proxy(ctx, id, false)
}

func (s *Store) Proxy(ctx context.Context, id int64, includeSecret bool) (Proxy, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, label, url_cipher, enabled, status, failure_count, last_used_at,
       last_success_at, last_error, cooldown_until, created_at, updated_at
FROM proxies WHERE id = ?`, id)
	proxy, cipherText, err := scanProxy(row)
	if err != nil {
		return Proxy{}, err
	}
	proxyURL, err := s.cipher.Decrypt(cipherText)
	if err != nil {
		return Proxy{}, err
	}
	proxy.URLSummary = summarizeProxyURL(proxyURL)
	if includeSecret {
		proxy.URL = proxyURL
	}
	return proxy, nil
}

func (s *Store) ListProxies(ctx context.Context) ([]Proxy, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, label, url_cipher, enabled, status, failure_count, last_used_at,
       last_success_at, last_error, cooldown_until, created_at, updated_at
FROM proxies ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	proxies := make([]Proxy, 0)
	for rows.Next() {
		proxy, cipherText, err := scanProxy(rows)
		if err != nil {
			return nil, err
		}
		proxyURL, err := s.cipher.Decrypt(cipherText)
		if err != nil {
			return nil, err
		}
		proxy.URLSummary = summarizeProxyURL(proxyURL)
		proxies = append(proxies, proxy)
	}
	return proxies, rows.Err()
}

func (s *Store) UpdateProxy(ctx context.Context, id int64, input ProxyUpdate) (Proxy, error) {
	input.Label = strings.TrimSpace(input.Label)
	if input.Label == "" {
		return Proxy{}, errors.New("proxy label is required")
	}
	current, err := s.Proxy(ctx, id, true)
	if err != nil {
		return Proxy{}, err
	}
	proxyURL := current.URL
	urlChanged := false
	if input.URL != nil {
		proxyURL = strings.TrimSpace(*input.URL)
		urlChanged = proxyURL != current.URL
	}
	if err := validateProxyURL(proxyURL); err != nil {
		return Proxy{}, err
	}
	sealed, err := s.cipher.Encrypt(proxyURL)
	if err != nil {
		return Proxy{}, err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE proxies SET label = ?, url_cipher = ?, enabled = ?, status = CASE WHEN ? THEN 'unknown' ELSE status END,
	                   failure_count = CASE WHEN ? THEN 0 ELSE failure_count END,
	                   cooldown_until = CASE WHEN ? THEN 0 ELSE cooldown_until END,
	                   last_error = CASE WHEN ? THEN '' ELSE last_error END, updated_at = ?
WHERE id = ?`, input.Label, sealed, boolInt(input.Enabled), urlChanged, urlChanged, urlChanged, urlChanged, unixNow(), id)
	if err != nil {
		return Proxy{}, err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return Proxy{}, sql.ErrNoRows
	}
	return s.Proxy(ctx, id, false)
}

func (s *Store) DeleteProxy(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET proxy_id = 0 WHERE proxy_id = ?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM proxies WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (s *Store) PickProxy(ctx context.Context, preferredID int64) (Proxy, bool, error) {
	now := unixNow()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Proxy{}, false, err
	}
	defer tx.Rollback()
	query := `
SELECT id, label, url_cipher, enabled, status, failure_count, last_used_at,
       last_success_at, last_error, cooldown_until, created_at, updated_at
FROM proxies WHERE enabled = 1 AND cooldown_until <= ?
ORDER BY CASE WHEN id = ? THEN 0 ELSE 1 END, last_used_at ASC, id ASC LIMIT 1`
	proxy, cipherText, err := scanProxy(tx.QueryRowContext(ctx, query, now, preferredID))
	if errors.Is(err, sql.ErrNoRows) {
		return Proxy{}, false, nil
	}
	if err != nil {
		return Proxy{}, false, err
	}
	proxyURL, err := s.cipher.Decrypt(cipherText)
	if err != nil {
		return Proxy{}, false, err
	}
	proxy.URL = proxyURL
	proxy.URLSummary = summarizeProxyURL(proxyURL)
	proxy.LastUsedAt = now
	if _, err := tx.ExecContext(ctx, `UPDATE proxies SET last_used_at = ?, updated_at = ? WHERE id = ?`, now, now, proxy.ID); err != nil {
		return Proxy{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Proxy{}, false, err
	}
	return proxy, true, nil
}

func (s *Store) ProxyCounts(ctx context.Context) (total, enabled int64, err error) {
	err = s.db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(CASE WHEN enabled = 1 THEN 1 ELSE 0 END), 0) FROM proxies`).Scan(&total, &enabled)
	return total, enabled, err
}

func (s *Store) ReportProxy(ctx context.Context, id int64, success bool, message string) error {
	if id == 0 {
		return nil
	}
	now := unixNow()
	if success {
		_, err := s.db.ExecContext(ctx, `
UPDATE proxies SET status = 'healthy', failure_count = 0, last_success_at = ?,
                   last_error = '', cooldown_until = 0, updated_at = ? WHERE id = ?`, now, now, id)
		return err
	}
	message = truncate(message, 300)
	_, err := s.db.ExecContext(ctx, `
UPDATE proxies SET status = CASE WHEN failure_count + 1 >= ? THEN 'cooldown' ELSE 'unhealthy' END,
                   failure_count = failure_count + 1,
                   cooldown_until = CASE WHEN failure_count + 1 >= ? THEN ? ELSE cooldown_until END,
                   last_error = ?, updated_at = ? WHERE id = ?`, proxyCircuitThreshold,
		proxyCircuitThreshold, now+int64((5*time.Minute).Seconds()), message, now, id)
	return err
}

func (s *Store) ResetProxy(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE proxies SET status = 'unknown', failure_count = 0, cooldown_until = 0,
                   last_error = '', updated_at = ? WHERE id = ?`, unixNow(), id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func scanProxy(row scanner) (Proxy, string, error) {
	var proxy Proxy
	var cipherText string
	var enabled int
	err := row.Scan(&proxy.ID, &proxy.Label, &cipherText, &enabled, &proxy.Status,
		&proxy.FailureCount, &proxy.LastUsedAt, &proxy.LastSuccessAt, &proxy.LastError,
		&proxy.CooldownUntil, &proxy.CreatedAt, &proxy.UpdatedAt)
	proxy.Enabled = enabled != 0
	return proxy, cipherText, err
}

func validateProxyURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return errors.New("proxy URL is invalid")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return nil
	default:
		return errors.New("proxy URL scheme must be http, https, socks5, or socks5h")
	}
}

func summarizeProxyURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil {
		return "configured"
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		if username == "" {
			username = "user"
		}
		parsed.User = url.UserPassword(username, "••••")
	}
	return parsed.String()
}
