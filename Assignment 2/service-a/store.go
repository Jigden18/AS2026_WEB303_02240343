package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"tutorsessions/gen/sessionpb"
)

var errNotFound = errors.New("session not found")

// Store wraps the Session database. It never touches tutor data:
// only tutor_id is stored.
type Store struct{ db *sql.DB }

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
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		tutor_id TEXT NOT NULL,
		student_name TEXT NOT NULL,
		session_date TEXT NOT NULL,
		duration_minutes INTEGER NOT NULL
	)`)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Create(ctx context.Context, x *sessionpb.Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, tutor_id, student_name, session_date, duration_minutes) VALUES ($1,$2,$3,$4,$5)`,
		x.Id, x.TutorId, x.StudentName, x.SessionDate, x.DurationMinutes)
	return err
}

func (s *Store) Get(ctx context.Context, id string) (*sessionpb.Session, error) {
	x := &sessionpb.Session{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, tutor_id, student_name, session_date, duration_minutes FROM sessions WHERE id=$1`, id).
		Scan(&x.Id, &x.TutorId, &x.StudentName, &x.SessionDate, &x.DurationMinutes)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errNotFound
	}
	return x, err
}

// Update changes student, date and duration (the tutor of a session is fixed).
func (s *Store) Update(ctx context.Context, x *sessionpb.Session) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET student_name=$2, session_date=$3, duration_minutes=$4 WHERE id=$1`,
		x.Id, x.StudentName, x.SessionDate, x.DurationMinutes)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}
