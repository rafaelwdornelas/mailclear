// Package handlers contém os HTTP handlers da API.
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
)

// ErrorResponse é o envelope padrão de erro retornado pela API.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// WriteError serializa um envelope de erro JSON com status code.
func WriteError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: msg, Code: code}); err != nil {
		// Headers já foram escritos; só dá pra observar via log.
		log.Warn().Err(err).Int("status", status).Str("code", code).Msg("WriteError encode falhou")
	}
}

// WriteJSON serializa qualquer valor como JSON.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Warn().Err(err).Int("status", status).Msg("WriteJSON encode falhou")
	}
}
