package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) CreateSession(ctx context.Context, session Session) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions(token_hash, csrf_token, expires_at, created_at) VALUES(?, ?, ?, ?)
ON CONFLICT(token_hash) DO UPDATE SET csrf_token = excluded.csrf_token, expires_at = excluded.expires_at`,
		session.TokenHash, session.CSRFToken, session.ExpiresAt, unixNow())
	return err
}

func (s *Store) Session(ctx context.Context, tokenHash string) (Session, bool, error) {
	var session Session
	err := s.db.QueryRowContext(ctx, `
SELECT token_hash, csrf_token, expires_at FROM sessions
WHERE token_hash = ? AND expires_at > ?`, tokenHash, unixNow()).Scan(&session.TokenHash, &session.CSRFToken, &session.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	return session, true, nil
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteAllSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions`)
	return err
}

func (s *Store) PurgeSessions(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, unixNow())
	return err
}
