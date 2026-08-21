// Package response centralises HTTP response writing so that every endpoint
// emits the same content types, the same error shape, and the same request
// correlation id. Errors are rendered in the dialect attached to the request,
// so a caller never needs to know whether it is serving the admin API or OFREP.
package response

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

const (
	contentTypeJSON    = "application/json; charset=utf-8"
	contentTypeProblem = "application/problem+json; charset=utf-8"
)

// Hand-rolled bodies for the case where marshalling the real one failed. They
// are constants so that emitting them cannot fail in turn.
const (
	problemFallback = `{"title":"Internal Server Error","status":500}`
	ofrepFallback   = `{"errorCode":"GENERAL","errorDetails":"an unexpected error occurred"}`
)

// Problem is an RFC 9457 problem details document, extended with the request
// id so a user-reported failure can be traced to a log line.
type Problem struct {
	Type      string `json:"type,omitempty"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	RequestID string `json:"requestId,omitempty"`
}

// APIError describes an error response independently of the wire format. The
// dialect on the request decides which fields survive: Type is problem details
// only, Code and Key are OFREP only. Detail should be safe to expose; internal
// causes belong in the log, not in the body.
type APIError struct {
	Status int
	Detail string
	Code   ErrorCode
	Key    string
	Type   string
}

// JSON marshals v and writes it with the given status code. Marshalling
// happens into a buffer first so that a failure can still produce a valid 500
// rather than a truncated body under an already-sent 200.
func JSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	write(w, r, status, contentTypeJSON, v)
}

// Error writes an error response in the request's dialect, carrying only a
// status and a detail. Callers with more to say should use WriteError.
func Error(w http.ResponseWriter, r *http.Request, status int, detail string) {
	WriteError(w, r, APIError{Status: status, Detail: detail})
}

// WriteError renders e in the request's dialect.
func WriteError(w http.ResponseWriter, r *http.Request, e APIError) {
	if e.Status == 0 {
		e.Status = http.StatusInternalServerError
	}

	if dialectFrom(r.Context()) == OFREP {
		writeOFREP(w, r, e)
		return
	}

	writeProblem(w, r, e)
}

// ErrorFrom maps a known error type to a response. http.MaxBytesReader surfaces
// oversized bodies here rather than in the body-limit middleware, because a
// chunked or under-declared request only fails once the handler reads it.
// Anything unrecognised is logged and reported as a 500.
func ErrorFrom(w http.ResponseWriter, r *http.Request, err error) {
	var maxBytes *http.MaxBytesError
	if errors.As(err, &maxBytes) {
		WriteError(w, r, APIError{
			Status: http.StatusRequestEntityTooLarge,
			Detail: fmt.Sprintf("request body exceeds %d bytes", maxBytes.Limit),
		})

		return
	}

	slog.ErrorContext(r.Context(), "unhandled request error",
		"error", err,
		"path", r.URL.Path,
		"request_id", middleware.GetReqID(r.Context()),
	)

	WriteError(w, r, APIError{
		Status: http.StatusInternalServerError,
		Detail: "an unexpected error occurred",
	})
}

func writeProblem(w http.ResponseWriter, r *http.Request, e APIError) {
	problem := Problem{
		Type:      e.Type,
		Title:     http.StatusText(e.Status),
		Status:    e.Status,
		Detail:    e.Detail,
		Instance:  r.URL.Path,
		RequestID: middleware.GetReqID(r.Context()),
	}

	write(w, r, e.Status, contentTypeProblem, problem)
}

func write(w http.ResponseWriter, r *http.Request, status int, contentType string, v any) {
	var buf bytes.Buffer

	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response body",
			"error", err,
			"path", r.URL.Path,
			"request_id", middleware.GetReqID(r.Context()),
		)

		writeFallback(w, r)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)

	// A failed write almost always means the client disconnected. There is no
	// response left to send, so record it and move on.
	if _, err := w.Write(buf.Bytes()); err != nil {
		slog.WarnContext(r.Context(), "failed to write response body",
			"error", err,
			"path", r.URL.Path,
			"request_id", middleware.GetReqID(r.Context()),
		)
	}
}

// writeFallback emits a minimal body when marshalling the real one failed. It
// keeps the content type promised by the dialect; net/http's Error helper would
// downgrade the response to text/plain and break the contract at the one moment
// a client most needs a parseable body.
func writeFallback(w http.ResponseWriter, r *http.Request) {
	body, contentType := problemFallback, contentTypeProblem
	if dialectFrom(r.Context()) == OFREP {
		body, contentType = ofrepFallback, contentTypeJSON
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusInternalServerError)

	_, _ = io.WriteString(w, body)
}
