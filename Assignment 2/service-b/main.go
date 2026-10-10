// Tutor service (Service B): owns the Tutor resource and its own database.
package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"tutorsessions/gen/tutorpb"
)

// mustEnv reads a required environment variable or stops the service.
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable %s", key)
	}
	return v
}

func main() {
	port := mustEnv("TUTOR_GRPC_PORT")
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		mustEnv("DB_HOST"), mustEnv("DB_PORT"), mustEnv("DB_USER"),
		mustEnv("DB_PASSWORD"), mustEnv("DB_NAME"))

	store, err := NewStore(dsn)
	if err != nil {
		log.Fatalf("database setup failed: %v", err)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen failed: %v", err)
	}

	s := grpc.NewServer()
	tutorpb.RegisterTutorServiceServer(s, &Server{store: store})
	log.Printf("tutor service listening on :%s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("serve failed: %v", err)
	}
}
