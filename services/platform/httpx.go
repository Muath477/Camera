package platform

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
)

// JSON writes v with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error writes {"error": msg}.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}

// Decode reads a JSON body into v, rejecting unknown fields.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// Fail maps an error to a response: not-found for missing rows, 500 otherwise.
func Fail(w http.ResponseWriter, r *http.Request, err error) {
	var he *HTTPError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		Error(w, http.StatusNotFound, "not found")
	case errors.As(err, &he):
		Error(w, he.Status, he.Msg)
	default:
		slog.Error("request failed", "path", r.URL.Path, "err", err)
		Error(w, http.StatusInternalServerError, "internal error")
	}
}

// HTTPError carries a status through module code.
type HTTPError struct {
	Status int
	Msg    string
}

func (e *HTTPError) Error() string { return e.Msg }

func Errorf(status int, msg string) error { return &HTTPError{Status: status, Msg: msg} }

var (
	ErrForbidden = Errorf(http.StatusForbidden, "forbidden")
	ErrBadInput  = Errorf(http.StatusBadRequest, "invalid input")
)
