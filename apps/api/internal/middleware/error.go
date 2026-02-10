package middleware

import (
	"encoding/json"
	"net/http"
)

type errorEnvelope struct {
	Error     errorBody `json:"error"`
	RequestID string    `json:"requestId"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorEnvelope{
		Error: errorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
		RequestID: RequestIDFromContext(r.Context()),
	})
}
