package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"proyecto-cursos/internal/platform/api"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	api.WriteJSON(w, status, payload)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	api.WriteError(w, status, api.CodeFromMessage(msg), msg)
}

func DecodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("request body is required")
		}
		return err
	}

	if decoder.More() {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}
