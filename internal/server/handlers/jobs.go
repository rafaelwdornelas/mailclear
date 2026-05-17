package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/rafaelwdornelas/mailclear/internal/jobs"
	"github.com/rafaelwdornelas/mailclear/internal/storage"
)

// JobsHandlers expõe endpoints de jobs.
type JobsHandlers struct {
	mgr     *jobs.Manager
	results *storage.ResultsRepo
}

// NewJobs cria os handlers.
func NewJobs(mgr *jobs.Manager, results *storage.ResultsRepo) *JobsHandlers {
	return &JobsHandlers{mgr: mgr, results: results}
}

// List devolve uma página de jobs.
func (h *JobsHandlers) List(w http.ResponseWriter, r *http.Request) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50, 200)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0, 0)
	list, err := h.mgr.List(r.Context(), int32(limit), int32(offset))
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"jobs": list, "limit": limit, "offset": offset})
}

// Get devolve o status de um job.
func (h *JobsHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_ID", "ID inválido")
		return
	}
	j, err := h.mgr.Get(r.Context(), id)
	if err != nil {
		WriteError(w, http.StatusNotFound, "NOT_FOUND", "Job não encontrado")
		return
	}
	WriteJSON(w, http.StatusOK, j)
}

// Cancel cancela um job.
func (h *JobsHandlers) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_ID", "ID inválido")
		return
	}
	if err := h.mgr.Cancel(r.Context(), id); err != nil {
		WriteError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// Results devolve resultados paginados (cursor por id).
func (h *JobsHandlers) Results(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_ID", "ID inválido")
		return
	}
	status := r.URL.Query().Get("status")
	afterID := int64(parseIntDefault(r.URL.Query().Get("after"), 0, 0))
	limit := int32(parseIntDefault(r.URL.Query().Get("limit"), 100, 1000))

	rows, err := h.mgr.Results(r.Context(), id, status, afterID, limit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	var nextCursor int64
	if len(rows) > 0 {
		nextCursor = rows[len(rows)-1].ID
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"results":     rows,
		"limit":       limit,
		"next_cursor": nextCursor,
	})
}

// ExportCSV faz stream do CSV completo.
func (h *JobsHandlers) ExportCSV(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_ID", "ID inválido")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="job_%s.csv"`, id.String()))

	if err := jobs.ExportCSV(r.Context(), h.results, id, w, 1000); err != nil {
		// Não dá pra alterar status code se já enviou bytes — só loga
		fmt.Fprintf(w, "\n# export error: %v\n", err)
	}
}

func parseIntDefault(s string, def, max int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return def
	}
	if max > 0 && v > max {
		return max
	}
	return v
}
