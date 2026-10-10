package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/sony/gobreaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"tutorsessions/gen/tutorpb"
)

// errTutorUnavailable means the Tutor service could not be reached
// (after retries, or because the circuit breaker is open).
var errTutorUnavailable = errors.New("tutor service unavailable")

// TutorClient calls Service B with a timeout, retries with backoff,
// and a circuit breaker.
type TutorClient struct {
	client      tutorpb.TutorServiceClient
	breaker     *gobreaker.CircuitBreaker
	timeout     time.Duration // per-attempt timeout
	maxRetries  int           // retries after the first attempt
	baseBackoff time.Duration // doubles on every retry
}

// isTransient reports errors worth retrying. Business errors such as
// NotFound or InvalidArgument are NOT retried and do NOT trip the breaker.
func isTransient(err error) bool {
	c := status.Code(err)
	return c == codes.Unavailable || c == codes.DeadlineExceeded
}

func NewTutorClient(addr string, timeout time.Duration, maxRetries int,
	failThreshold uint32, openFor time.Duration) (*TutorClient, error) {

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "tutor-service",
		MaxRequests: 1,                // one trial call while half-open
		Interval:    30 * time.Second, // failure counts reset after this
		Timeout:     openFor,          // how long the breaker stays open
		ReadyToTrip: func(c gobreaker.Counts) bool {
			return c.ConsecutiveFailures >= failThreshold
		},
		IsSuccessful: func(err error) bool { return err == nil || !isTransient(err) },
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("CIRCUIT BREAKER %s: %s -> %s", name, from, to)
		},
	})
	return &TutorClient{
		client:      tutorpb.NewTutorServiceClient(conn),
		breaker:     cb,
		timeout:     timeout,
		maxRetries:  maxRetries,
		baseBackoff: 100 * time.Millisecond,
	}, nil
}

// GetTutor fetches a tutor from Service B. It returns the gRPC error as-is for
// business errors (NotFound...) and errTutorUnavailable for outages.
func (c *TutorClient) GetTutor(ctx context.Context, id string) (*tutorpb.Tutor, error) {
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			wait := c.baseBackoff << (attempt - 1) // 100ms, 200ms, 400ms...
			log.Printf("retry %d/%d for tutor %s in %v", attempt, c.maxRetries, id, wait)
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return nil, errTutorUnavailable
			}
		}

		res, err := c.breaker.Execute(func() (interface{}, error) {
			callCtx, cancel := context.WithTimeout(ctx, c.timeout) // timeout
			defer cancel()
			return c.client.GetTutor(callCtx, &tutorpb.GetTutorRequest{Id: id})
		})
		if err == nil {
			return res.(*tutorpb.Tutor), nil
		}
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			log.Printf("circuit breaker open: failing fast for tutor %s", id)
			return nil, errTutorUnavailable
		}
		if !isTransient(err) {
			return nil, err // NotFound, InvalidArgument: no retry
		}
		log.Printf("attempt %d to call tutor service failed: %v", attempt+1, err)
	}
	return nil, errTutorUnavailable
}
