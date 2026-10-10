// Session service (Service A): owns the Session resource and its own database.
// It calls the Tutor service (Service B) over gRPC with resilience patterns.
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"tutorsessions/gen/sessionpb"
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable %s", key)
	}
	return v
}

// envInt reads an optional integer variable, using def when unset.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func main() {
	port := mustEnv("SESSION_GRPC_PORT")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		mustEnv("DB_HOST"), mustEnv("DB_PORT"), mustEnv("DB_USER"),
		mustEnv("DB_PASSWORD"), mustEnv("DB_NAME"))

	store, err := NewStore(dsn)
	if err != nil {
		log.Fatalf("database setup failed: %v", err)
	}

	// Resilience settings (defaults: 2s timeout, 2 retries, trip after 5 failures, open 10s)
	tutors, err := NewTutorClient(mustEnv("TUTOR_SERVICE_ADDR"),
		time.Duration(envInt("TUTOR_TIMEOUT_MS", 2000))*time.Millisecond,
		envInt("TUTOR_MAX_RETRIES", 2),
		uint32(envInt("CB_FAILURE_THRESHOLD", 5)),
		time.Duration(envInt("CB_OPEN_SECONDS", 10))*time.Second)
	if err != nil {
		log.Fatalf("tutor client setup failed: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}
	s := grpc.NewServer()
	sessionpb.RegisterSessionServiceServer(s, &Server{store: store, tutors: tutors})
	log.Printf("session service listening on :%s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("serve failed: %v", err)
	}
}
