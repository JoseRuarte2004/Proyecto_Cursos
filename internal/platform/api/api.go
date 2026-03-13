package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"proyecto-cursos/internal/platform/requestid"
)

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type ErrorResponse struct {
	Error     ErrorPayload `json:"error"`
	RequestID string       `json:"requestId,omitempty"`
}

var nonAlphaNum = regexp.MustCompile(`[^A-Z0-9]+`)

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func WriteError(w http.ResponseWriter, status int, code, message string, details ...any) {
	var detail any
	if len(details) > 0 {
		detail = details[0]
	}

	WriteJSON(w, status, ErrorResponse{
		Error: ErrorPayload{
			Code:    normalizeCode(code, status, message),
			Message: strings.TrimSpace(message),
			Details: detail,
		},
		RequestID: strings.TrimSpace(w.Header().Get(requestid.Header)),
	})
}

func CodeFromMessage(message string) string {
	return normalizeCode("", http.StatusInternalServerError, message)
}

func normalizeCode(code string, status int, message string) string {
	code = strings.TrimSpace(code)
	if code != "" {
		return strings.ToUpper(nonAlphaNum.ReplaceAllString(code, "_"))
	}

	base := strings.TrimSpace(message)
	if base == "" {
		base = http.StatusText(status)
	}
	if base == "" {
		base = "INTERNAL_SERVER_ERROR"
	}

	base = strings.ToUpper(base)
	base = nonAlphaNum.ReplaceAllString(base, "_")
	base = strings.Trim(base, "_")
	if base == "" {
		return "INTERNAL_SERVER_ERROR"
	}

	return base
}
