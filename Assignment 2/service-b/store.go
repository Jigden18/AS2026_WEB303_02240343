package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"tutorsessions/gen/tutorpb"
)

var errNotFound = errors.New("tutor not found")

// Store wraps the Tutor database (the only place SQL lives).
type Store struct{ db *sql.DB }

// NewStore connects (retrying while the DB container starts) and creates the table.
func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 30; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("database not reachable: %w", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tutors (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT NOT NULL,
		subject TEXT NOT NULL,
		hourly_rate DOUBLE PRECISION NOT NULL
	)`)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Create(ctx context.Context, t *tutorpb.Tutor) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tutors (id, name, email, subject, hourly_rate) VALUES ($1,$2,$3,$4,$5)`,
		t.Id, t.Name, t.Email, t.Subject, t.HourlyRate)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (*tutorpb.Tutor, error) {
	t := &tutorpb.Tutor{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, email, subject, hourly_rate FROM tutors WHERE id=$1`, id).
		Scan(&t.Id, &t.Name, &t.Email, &t.Subject, &t.HourlyRate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	return t, err
}

func (s *Store) Update(ctx context.Context, t *tutorpb.Tutor) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE tutors SET name=$2, email=$3, subject=$4, hourly_rate=$5 WHERE id=$1`,
		t.Id, t.Name, t.Email, t.Subject, t.HourlyRate)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM tutors WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}
