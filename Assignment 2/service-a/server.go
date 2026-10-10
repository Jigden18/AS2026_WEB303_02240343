package main

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"tutorsessions/gen/sessionpb"
)

// Server implements sessionpb.SessionServiceServer.
type Server struct {
	sessionpb.UnimplementedSessionServiceServer
	store  *Store
	tutors *TutorClient
}

func validateUUID(field, v string) error {
	if _, err := uuid.Parse(v); err != nil {
		return status.Errorf(codes.InvalidArgument, "%s must be a valid UUID", field)
	}
	return nil
}

// validateFields checks student name, date format (YYYY-MM-DD) and duration.
func validateFields(student, date string, duration int32) error {
	if strings.TrimSpace(student) == "" {
		return status.Error(codes.InvalidArgument, "student_name is required")
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return status.Error(codes.InvalidArgument, "session_date must be in YYYY-MM-DD format")
	}
	if duration < 15 || duration > 480 {
		return status.Error(codes.InvalidArgument, "duration_minutes must be between 15 and 480")
	}
	return nil
}

func toStatus(err error) error {
	if errors.Is(err, errNotFound) {
		return status.Error(codes.NotFound, "session not found")
	}
	log.Printf("internal error: %v", err)
	return status.Error(codes.Internal, "internal error")
}

// addTutorName calls Service B to fill in tutor_name. If Service B is down it
// uses a FALLBACK value instead of failing the whole request.
func (s *Server) addTutorName(ctx context.Context, x *sessionpb.Session) {
	t, err := s.tutors.GetTutor(ctx, x.TutorId)
	switch {
	case err == nil:
		x.TutorName = t.Name
	case errors.Is(err, errTutorUnavailable):
		x.TutorName = "unavailable (tutor service is down)"
	case status.Code(err) == codes.NotFound:
		x.TutorName = "unknown (tutor no longer exists)"
	default:
		x.TutorName = "unavailable"
	}
}

// CreateSession validates input, then CONFIRMS the tutor exists via Service B.
func (s *Server) CreateSession(ctx context.Context, req *sessionpb.CreateSessionRequest) (*sessionpb.Session, error) {
	if err := validateUUID("tutor_id", req.TutorId); err != nil {
		return nil, err
	}
	if err := validateFields(req.StudentName, req.SessionDate, req.DurationMinutes); err != nil {
		return nil, err
	}

	tutor, err := s.tutors.GetTutor(ctx, req.TutorId)
	if err != nil {
		switch {
		case errors.Is(err, errTutorUnavailable):
			return nil, status.Error(codes.Unavailable, "tutor service is unavailable, please try again later")
		case status.Code(err) == codes.NotFound:
			return nil, status.Error(codes.NotFound, "tutor does not exist")
		default:
			log.Printf("tutor lookup error: %v", err)
			return nil, status.Error(codes.Internal, "internal error")
		}
	}

	x := &sessionpb.Session{Id: uuid.NewString(), TutorId: req.TutorId, TutorName: tutor.Name,
		StudentName: req.StudentName, SessionDate: req.SessionDate, DurationMinutes: req.DurationMinutes}
	if err := s.store.Create(ctx, x); err != nil {
		return nil, toStatus(err)
	}
	return x, nil
}

func (s *Server) GetSession(ctx context.Context, req *sessionpb.GetSessionRequest) (*sessionpb.Session, error) {
	if err := validateUUID("id", req.Id); err != nil {
		return nil, err
	}
	x, err := s.store.Get(ctx, req.Id)
	if err != nil {
		return nil, toStatus(err)
	}
	s.addTutorName(ctx, x)
	return x, nil
}

func (s *Server) UpdateSession(ctx context.Context, req *sessionpb.UpdateSessionRequest) (*sessionpb.Session, error) {
	if err := validateUUID("id", req.Id); err != nil {
		return nil, err
	}
	if err := validateFields(req.StudentName, req.SessionDate, req.DurationMinutes); err != nil {
		return nil, err
	}
	x := &sessionpb.Session{Id: req.Id, StudentName: req.StudentName,
		SessionDate: req.SessionDate, DurationMinutes: req.DurationMinutes}
	if err := s.store.Update(ctx, x); err != nil {
		return nil, toStatus(err)
	}
	updated, err := s.store.Get(ctx, req.Id)
	if err != nil {
		return nil, toStatus(err)
	}
	s.addTutorName(ctx, updated)
	return updated, nil
}

func (s *Server) DeleteSession(ctx context.Context, req *sessionpb.DeleteSessionRequest) (*sessionpb.DeleteSessionResponse, error) {
	if err := validateUUID("id", req.Id); err != nil {
		return nil, err
	}
	if err := s.store.Delete(ctx, req.Id); err != nil {
		return nil, toStatus(err)
	}
	return &sessionpb.DeleteSessionResponse{Success: true}, nil
}
