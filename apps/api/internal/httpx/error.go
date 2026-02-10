package httpx

import (
	"net/http"

	"github.com/moveops-platform/apps/api/internal/middleware"
)

type ErrorEnvelope struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"requestId"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	WriteJSON(w, status, ErrorEnvelope{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
		RequestID: middleware.RequestIDFromContext(r.Context()),
	})
}
