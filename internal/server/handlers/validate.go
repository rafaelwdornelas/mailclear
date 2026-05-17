package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rafaelwdornelas/mailclear/internal/validation"
)

// ValidateHandlers expõe endpoints síncronos de validação.
type ValidateHandlers struct {
	val validation.Validator
}

// NewValidate cria os handlers.
func NewValidate(val validation.Validator) *ValidateHandlers {
	return &ValidateHandlers{val: val}
}

// SingleRequest body do POST /validate.
type SingleRequest struct {
	Email string `json:"email"`
}

// BatchRequest body do POST /validate/batch.
type BatchRequest struct {
	Emails []string `json:"emails"`
}

// Single processa um único email síncrono.
func (h *ValidateHandlers) Single(w http.ResponseWriter, r *http.Request) {
	var req SingleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_BODY", "JSON inválido")
		return
	}
	if req.Email == "" {
		WriteError(w, http.StatusBadRequest, "MISSING_EMAIL", "Campo 'email' obrigatório")
		return
	}
	res, err := h.val.Validate(r.Context(), req.Email)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "VALIDATION_ERROR", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, res)
}

// Batch processa um lote síncrono.
func (h *ValidateHandlers) Batch(w http.ResponseWriter, r *http.Request) {
	var req BatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_BODY", "JSON inválido")
		return
	}
	if len(req.Emails) == 0 {
		WriteError(w, http.StatusBadRequest, "MISSING_EMAILS", "Campo 'emails' obrigatório")
		return
	}
	const maxBatch = 1000
	if len(req.Emails) > maxBatch {
		WriteError(w, http.StatusRequestEntityTooLarge, "BATCH_TOO_LARGE",
			"Batch síncrono limitado a 1000 emails — use /import/csv para volumes maiores")
		return
	}
	out, err := h.val.ValidateBatch(r.Context(), req.Emails)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "VALIDATION_ERROR", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"results": out, "total": len(out)})
}
