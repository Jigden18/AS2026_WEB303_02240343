package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"tutorsessions/gen/sessionpb"
	"tutorsessions/gen/tutorpb"
)

// Gateway holds the gRPC clients used by the REST handlers.
type Gateway struct {
	tutors   tutorpb.TutorServiceClient
	sessions sessionpb.SessionServiceClient
	timeout  time.Duration
}

// JSON request bodies (the services do the real validation).
type tutorBody struct {
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	Subject    string  `json:"subject"`
	HourlyRate float64 `json:"hourly_rate"`
}

type sessionBody struct {
	TutorID         string `json:"tutor_id"` // used on create only
	StudentName     string `json:"student_name"`
	SessionDate     string `json:"session_date"`
	DurationMinutes int32  `json:"duration_minutes"`
}

var jsonOut = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}

// ---------- helpers ----------

func (g *Gateway) ctx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), g.timeout)
}

func writeJSON(w http.ResponseWriter, code int, msg proto.Message) {
	raw, err := jsonOut.Marshal(msg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var pretty bytes.Buffer
	json.Indent(&pretty, raw, "", "  ")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(pretty.Bytes())
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// httpStatus maps gRPC status codes to HTTP status codes.
func httpStatus(c codes.Code) int {
	switch c {
	case codes.InvalidArgument:
		return http.StatusBadRequest // 400
	case codes.NotFound:
		return http.StatusNotFound // 404
	case codes.AlreadyExists:
		return http.StatusConflict // 409
	case codes.Unavailable:
		return http.StatusServiceUnavailable // 503
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout // 504
	default:
		return http.StatusInternalServerError // 500
	}
}

func writeGRPCError(w http.ResponseWriter, err error) {
	st, _ := status.FromError(err)
	code := httpStatus(st.Code())
	if code == http.StatusInternalServerError {
		log.Printf("gateway: unexpected error: %v", err)
		writeError(w, code, "internal server error")
		return
	}
	writeError(w, code, st.Message())
}

// decode reads a JSON body; malformed JSON or unknown fields give 400.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

// ---------- tutor handlers ----------

func (g *Gateway) createTutor(w http.ResponseWriter, r *http.Request) {
	var b tutorBody
	if !decode(w, r, &b) {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()
	res, err := g.tutors.CreateTutor(ctx, &tutorpb.CreateTutorRequest{
		Name: b.Name, Email: b.Email, Subject: b.Subject, HourlyRate: b.HourlyRate})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.Header().Set("Location", "/tutors/"+res.Id)
	writeJSON(w, http.StatusCreated, res)
}

func (g *Gateway) getTutor(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.ctx(r)
	defer cancel()
	res, err := g.tutors.GetTutor(ctx, &tutorpb.GetTutorRequest{Id: r.PathValue("id")})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (g *Gateway) updateTutor(w http.ResponseWriter, r *http.Request) {
	var b tutorBody
	if !decode(w, r, &b) {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()
	res, err := g.tutors.UpdateTutor(ctx, &tutorpb.UpdateTutorRequest{Id: r.PathValue("id"),
		Name: b.Name, Email: b.Email, Subject: b.Subject, HourlyRate: b.HourlyRate})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (g *Gateway) deleteTutor(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.ctx(r)
	defer cancel()
	if _, err := g.tutors.DeleteTutor(ctx, &tutorpb.DeleteTutorRequest{Id: r.PathValue("id")}); err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent) // 204
}

// ---------- session handlers ----------

func (g *Gateway) createSession(w http.ResponseWriter, r *http.Request) {
	var b sessionBody
	if !decode(w, r, &b) {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()
	res, err := g.sessions.CreateSession(ctx, &sessionpb.CreateSessionRequest{TutorId: b.TutorID,
		StudentName: b.StudentName, SessionDate: b.SessionDate, DurationMinutes: b.DurationMinutes})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	w.Header().Set("Location", "/sessions/"+res.Id)
	writeJSON(w, http.StatusCreated, res)
}

func (g *Gateway) getSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.ctx(r)
	defer cancel()
	res, err := g.sessions.GetSession(ctx, &sessionpb.GetSessionRequest{Id: r.PathValue("id")})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (g *Gateway) updateSession(w http.ResponseWriter, r *http.Request) {
	var b sessionBody
	if !decode(w, r, &b) {
		return
	}
	ctx, cancel := g.ctx(r)
	defer cancel()
	res, err := g.sessions.UpdateSession(ctx, &sessionpb.UpdateSessionRequest{Id: r.PathValue("id"),
		StudentName: b.StudentName, SessionDate: b.SessionDate, DurationMinutes: b.DurationMinutes})
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (g *Gateway) deleteSession(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := g.ctx(r)
	defer cancel()
	if _, err := g.sessions.DeleteSession(ctx, &sessionpb.DeleteSessionRequest{Id: r.PathValue("id")}); err != nil {
		writeGRPCError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent) // 204
}
