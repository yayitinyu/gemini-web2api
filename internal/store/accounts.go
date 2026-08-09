package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

func (s *Store) CreateAccount(ctx context.Context, input AccountInput) (Account, error) {
	input.Label = strings.TrimSpace(input.Label)
	input.Cookie = normalizeCookie(input.Cookie)
	if input.Label == "" {
		return Account{}, errors.New("account label is required")
	}
	if input.Cookie == "" {
		return Account{}, errors.New("cookie is required")
	}
	sealed, err := s.cipher.Encrypt(input.Cookie)
	if err != nil {
		return Account{}, err
	}
	now := unixNow()
	result, err := s.db.ExecContext(ctx, `
INSERT INTO accounts(label, cookie_cipher, enabled, status, note, proxy_id, created_at, updated_at)
VALUES(?, ?, ?, 'unknown', ?, ?, ?, ?)`, input.Label, sealed, boolInt(input.Enabled), strings.TrimSpace(input.Note), input.ProxyID, now, now)
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	id, _ := result.LastInsertId()
	return s.Account(ctx, id, false)
}

func (s *Store) Account(ctx context.Context, id int64, includeSecret bool) (Account, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, label, cookie_cipher, enabled, status, note, proxy_id, failure_count,
       last_used_at, last_success_at, last_error, created_at, updated_at
FROM accounts WHERE id = ?`, id)
	account, cipherText, err := scanAccount(row)
	if err != nil {
		return Account{}, err
	}
	cookie, err := s.cipher.Decrypt(cipherText)
	if err != nil {
		return Account{}, err
	}
	account.CookieSummary = summarizeCookie(cookie)
	if includeSecret {
		account.Cookie = cookie
	}
	return account, nil
}

func (s *Store) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, label, cookie_cipher, enabled, status, note, proxy_id, failure_count,
       last_used_at, last_success_at, last_error, created_at, updated_at
FROM accounts ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := make([]Account, 0)
	for rows.Next() {
		account, cipherText, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		cookie, err := s.cipher.Decrypt(cipherText)
		if err != nil {
			return nil, err
		}
		account.CookieSummary = summarizeCookie(cookie)
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

func (s *Store) UpdateAccount(ctx context.Context, id int64, input AccountUpdate) (Account, error) {
	input.Label = strings.TrimSpace(input.Label)
	if input.Label == "" {
		return Account{}, errors.New("account label is required")
	}
	current, err := s.Account(ctx, id, true)
	if err != nil {
		return Account{}, err
	}
	cookie := current.Cookie
	cookieChanged := false
	if input.Cookie != nil {
		cookie = normalizeCookie(*input.Cookie)
		if cookie == "" {
			return Account{}, errors.New("cookie cannot be empty")
		}
		cookieChanged = cookie != current.Cookie
	}
	sealed, err := s.cipher.Encrypt(cookie)
	if err != nil {
		return Account{}, err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE accounts SET label = ?, cookie_cipher = ?, enabled = ?, note = ?, proxy_id = ?,
	                    status = CASE WHEN ? THEN 'unknown' ELSE status END,
	                    failure_count = CASE WHEN ? THEN 0 ELSE failure_count END,
	                    last_error = CASE WHEN ? THEN '' ELSE last_error END,
	                    updated_at = ?
WHERE id = ?`, input.Label, sealed, boolInt(input.Enabled), strings.TrimSpace(input.Note), input.ProxyID,
		cookieChanged, cookieChanged, cookieChanged, unixNow(), id)
	if err != nil {
		return Account{}, err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return Account{}, sql.ErrNoRows
	}
	return s.Account(ctx, id, false)
}

func (s *Store) DeleteAccount(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) PickAccount(ctx context.Context) (Account, bool, error) {
	return s.PickAccountExcluding(ctx, nil)
}

func (s *Store) PickAccountExcluding(ctx context.Context, excluded map[int64]struct{}) (Account, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Account{}, false, err
	}
	defer tx.Rollback()
	query := `
SELECT id, label, cookie_cipher, enabled, status, note, proxy_id, failure_count,
       last_used_at, last_success_at, last_error, created_at, updated_at
FROM accounts WHERE enabled = 1`
	args := make([]any, 0, len(excluded))
	if len(excluded) > 0 {
		placeholders := make([]string, 0, len(excluded))
		for id := range excluded {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		query += " AND id NOT IN (" + strings.Join(placeholders, ",") + ")"
	}
	query += " ORDER BY last_used_at ASC, id ASC LIMIT 1"
	row := tx.QueryRowContext(ctx, query, args...)
	account, cipherText, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, false, nil
	}
	if err != nil {
		return Account{}, false, err
	}
	cookie, err := s.cipher.Decrypt(cipherText)
	if err != nil {
		return Account{}, false, err
	}
	account.Cookie = cookie
	account.CookieSummary = summarizeCookie(cookie)
	account.LastUsedAt = unixNow()
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET last_used_at = ?, updated_at = ? WHERE id = ?`, account.LastUsedAt, account.LastUsedAt, account.ID); err != nil {
		return Account{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Account{}, false, err
	}
	return account, true, nil
}

func (s *Store) AccountCounts(ctx context.Context) (total, enabled int64, err error) {
	err = s.db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(CASE WHEN enabled = 1 THEN 1 ELSE 0 END), 0) FROM accounts`).Scan(&total, &enabled)
	return total, enabled, err
}

func (s *Store) UpdateAccountCookie(ctx context.Context, id int64, cookie string) error {
	sealed, err := s.cipher.Encrypt(normalizeCookie(cookie))
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE accounts SET cookie_cipher = ?, updated_at = ? WHERE id = ?`, sealed, unixNow(), id)
	return err
}

func (s *Store) ReportAccount(ctx context.Context, id int64, success bool, message string) error {
	if id == 0 {
		return nil
	}
	message = truncate(message, 300)
	if success {
		_, err := s.db.ExecContext(ctx, `
UPDATE accounts SET status = 'healthy', failure_count = 0, last_success_at = ?, last_error = '', updated_at = ?
WHERE id = ?`, unixNow(), unixNow(), id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE accounts SET status = 'unhealthy', failure_count = failure_count + 1,
                    last_error = ?, updated_at = ? WHERE id = ?`, message, unixNow(), id)
	return err
}

func (s *Store) BindAccountProxy(ctx context.Context, accountID, proxyID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET proxy_id = ?, updated_at = ? WHERE id = ?`, proxyID, unixNow(), accountID)
	return err
}

type scanner interface{ Scan(...any) error }

func scanAccount(row scanner) (Account, string, error) {
	var account Account
	var cipherText string
	var enabled int
	err := row.Scan(&account.ID, &account.Label, &cipherText, &enabled, &account.Status, &account.Note,
		&account.ProxyID, &account.FailureCount, &account.LastUsedAt, &account.LastSuccessAt,
		&account.LastError, &account.CreatedAt, &account.UpdatedAt)
	account.Enabled = enabled != 0
	return account, cipherText, err
}

func normalizeCookie(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") {
		var wrapper struct {
			Cookie string `json:"cookie"`
		}
		if err := json.Unmarshal([]byte(value), &wrapper); err != nil || strings.TrimSpace(wrapper.Cookie) == "" {
			return ""
		}
		value = wrapper.Cookie
	}
	parts := strings.Split(value, ";")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" && strings.Contains(part, "=") {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, "; ")
}

func summarizeCookie(cookie string) string {
	parts := strings.Split(cookie, ";")
	names := make([]string, 0, len(parts))
	sapisidSuffix := ""
	for _, part := range parts {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || name == "" {
			continue
		}
		names = append(names, name)
		if name == "SAPISID" && len(value) >= 4 {
			sapisidSuffix = value[len(value)-4:]
		}
	}
	sort.Strings(names)
	critical := make([]string, 0, 3)
	for _, target := range []string{"SID", "__Secure-1PSID", "SAPISID"} {
		if sort.SearchStrings(names, target) < len(names) && names[sort.SearchStrings(names, target)] == target {
			critical = append(critical, target)
		}
	}
	summary := fmt.Sprintf("%d cookies · %s", len(names), strings.Join(critical, "/"))
	if sapisidSuffix != "" {
		summary += " · SAPISID ••••" + sapisidSuffix
	}
	return strings.TrimSuffix(summary, " · ")
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
