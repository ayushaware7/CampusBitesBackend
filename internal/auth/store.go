package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db} }

// ByUsername returns found=false (and no error) for an unknown user.
func (s *Store) ByUsername(ctx context.Context, username string) (hash string, canteenID int64, found bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT password_hash, canteen_id FROM admins WHERE username=?`, username).Scan(&hash, &canteenID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, nil
	}
	return hash, canteenID, err == nil, err
}

func (s *Store) Count(ctx context.Context) (n int, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admins`).Scan(&n)
	return
}

func (s *Store) Create(ctx context.Context, username, hash, canteenSlug string) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO admins (username, password_hash, canteen_id) SELECT ?, ?, id FROM canteens WHERE slug=?`,
		username, hash, canteenSlug)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("canteen %q does not exist", canteenSlug)
	}
	return nil
}
