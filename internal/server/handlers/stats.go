package handlers

import (
	"net/http"

	"github.com/rafaelwdornelas/mailclear/internal/storage"
)

// StatsHandlers expõe agregados globais.
type StatsHandlers struct {
	results *storage.ResultsRepo
}

// NewStats cria os handlers.
func NewStats(results *storage.ResultsRepo) *StatsHandlers {
	return &StatsHandlers{results: results}
}

// Global devolve estatísticas globais.
func (h *StatsHandlers) Global(w http.ResponseWriter, r *http.Request) {
	s, err := h.results.Global(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, s)
}
