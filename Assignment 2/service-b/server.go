package main

import (
	"context"
	"errors"
	"log"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"tutorsessions/gen/tutorpb"
)

// Server implements tutorpb.TutorServiceServer.
type Server struct {
	tutorpb.UnimplementedTutorServiceServer
	store *Store
}

// validateTutor rejects bad input with InvalidArgument (nothing is stored).
func validateTutor(name, email, subject string, rate float64) error {
	if strings.TrimSpace(name) == "" {
		return status.Error(codes.InvalidArgument, "name is required")
	}
	if strings.TrimSpace(subject) == "" {
		return status.Error(codes.InvalidArgument, "subject is required")
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return status.Error(codes.InvalidArgument, "a valid email is required")
	}
	if rate <= 0 {
		return status.Error(codes.InvalidArgument, "hourly_rate must be greater than 0")
	}
	return nil
}

func validateID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return status.Error(codes.InvalidArgument, "id must be a valid UUID")
	}
	return nil
}

// toStatus converts store errors into gRPC status errors.
func toStatus(err error) error {
	if errors.Is(err, errNotFound) {
		return status.Error(codes.NotFound, "tutor not found")
	}
	log.Printf("internal error: %v", err)
	return status.Error(codes.Internal, "internal error")
}

func (s *Server) CreateTutor(ctx context.Context, req *tutorpb.CreateTutorRequest) (*tutorpb.Tutor, error) {
	if err := validateTutor(req.Name, req.Email, req.Subject, req.HourlyRate); err != nil {
		return nil, err
	}
	t := &tutorpb.Tutor{Id: uuid.NewString(), Name: req.Name, Email: req.Email,
		Subject: req.Subject, HourlyRate: req.HourlyRate}
	if err := s.store.Create(ctx, t); err != nil {
		return nil, toStatus(err)
	}
	return t, nil
}

func (s *Server) GetTutor(ctx context.Context, req *tutorpb.GetTutorRequest) (*tutorpb.Tutor, error) {
	if err := validateID(req.Id); err != nil {
		return nil, err
	}
	t, err := s.store.Get(ctx, req.Id)
	if err != nil {
		return nil, toStatus(err)
	}
	return t, nil
}

func (s *Server) UpdateTutor(ctx context.Context, req *tutorpb.UpdateTutorRequest) (*tutorpb.Tutor, error) {
	if err := validateID(req.Id); err != nil {
		return nil, err
	}
	if err := validateTutor(req.Name, req.Email, req.Subject, req.HourlyRate); err != nil {
		return nil, err
	}
	t := &tutorpb.Tutor{Id: req.Id, Name: req.Name, Email: req.Email,
		Subject: req.Subject, HourlyRate: req.HourlyRate}
	if err := s.store.Update(ctx, t); err != nil {
		return nil, toStatus(err)
	}
	return t, nil
}

func (s *Server) DeleteTutor(ctx context.Context, req *tutorpb.DeleteTutorRequest) (*tutorpb.DeleteTutorResponse, error) {
	if err := validateID(req.Id); err != nil {
		return nil, err
	}
	if err := s.store.Delete(ctx, req.Id); err != nil {
		return nil, toStatus(err)
	}
	return &tutorpb.DeleteTutorResponse{Success: true}, nil
}
