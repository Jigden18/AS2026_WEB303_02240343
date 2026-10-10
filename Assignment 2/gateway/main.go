// API Gateway: the single REST entry point. It translates REST requests
// into gRPC calls to the Tutor and Session services.
package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"tutorsessions/gen/sessionpb"
	"tutorsessions/gen/tutorpb"
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required environment variable %s", key)
	}
	return v
}

func main() {
	port := mustEnv("GATEWAY_PORT")

	timeout := 10 // seconds; must exceed Service A's worst-case retry time
	if n, err := strconv.Atoi(os.Getenv("GATEWAY_TIMEOUT_SECONDS")); err == nil && n > 0 {
		timeout = n
	}

	creds := grpc.WithTransportCredentials(insecure.NewCredentials())
	tutorConn, err := grpc.NewClient(mustEnv("TUTOR_SERVICE_ADDR"), creds)
	if err != nil {
		log.Fatalf("tutor connection failed: %v", err)
	}
	sessionConn, err := grpc.NewClient(mustEnv("SESSION_SERVICE_ADDR"), creds)
	if err != nil {
		log.Fatalf("session connection failed: %v", err)
	}

	g := &Gateway{
		tutors:   tutorpb.NewTutorServiceClient(tutorConn),
		sessions: sessionpb.NewSessionServiceClient(sessionConn),
		timeout:  time.Duration(timeout) * time.Second,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	// Tutors (Service B)
	mux.HandleFunc("POST /tutors", g.createTutor)
	mux.HandleFunc("GET /tutors/{id}", g.getTutor)
	mux.HandleFunc("PUT /tutors/{id}", g.updateTutor)
	mux.HandleFunc("DELETE /tutors/{id}", g.deleteTutor)
	// Sessions (Service A)
	mux.HandleFunc("POST /sessions", g.createSession)
	mux.HandleFunc("GET /sessions/{id}", g.getSession)
	mux.HandleFunc("PUT /sessions/{id}", g.updateSession)
	mux.HandleFunc("DELETE /sessions/{id}", g.deleteSession)

	log.Printf("api gateway listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
