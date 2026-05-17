package handlers

import (
	"net/http"
	"strconv"

	"github.com/rafaelwdornelas/mailclear/internal/importer"
	"github.com/rafaelwdornelas/mailclear/internal/jobs"
)

// ImportHandlers expõe endpoints de upload massivo.
type ImportHandlers struct {
	mgr       *jobs.Manager
	batchSize int
}

// NewImport cria os handlers.
func NewImport(mgr *jobs.Manager, batchSize int) *ImportHandlers {
	if batchSize <= 0 {
		batchSize = 1000
	}
	return &ImportHandlers{mgr: mgr, batchSize: batchSize}
}

// CSV recebe um upload multipart com um arquivo CSV (campo "file"),
// cria um job e enfileira os emails para processamento assíncrono.
func (h *ImportHandlers) CSV(w http.ResponseWriter, r *http.Request) {
	// Backpressure: se a fila do pool está ≥90% cheia, recusa novos
	// uploads com 503 + Retry-After. Cliente que respeita HTTP padrão
	// vai esperar e tentar de novo. Evita acumular jobs até OOM.
	const queueBusyThreshold = 0.9
	if usage := h.mgr.QueueUsage(); usage >= queueBusyThreshold {
		w.Header().Set("Retry-After", "30")
		WriteError(w, http.StatusServiceUnavailable, "QUEUE_FULL",
			"Servidor ocupado (fila em "+strconv.Itoa(int(usage*100))+"%). Tente em 30s.")
		return
	}

	// Limita upload total a 512MB (memory map; em disco usa /tmp via PrivateTmp).
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_UPLOAD", "Upload inválido: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "MISSING_FILE", "Campo 'file' obrigatório")
		return
	}
	defer file.Close()

	jobName := header.Filename
	if jobName == "" {
		jobName = "upload.csv"
	}

	// Lê o CSV inteiro com dedup ANTES de criar o job — assim ele já nasce
	// com `total` correto, o front consegue mostrar progresso real, e o
	// completion tracker sabe quando marcar como completed.
	dedup := importer.NewDedup()
	var collected []string
	reader := importer.NewReader(file, ',')

	if err := reader.Each(r.Context(), func(email string) error {
		if dedup.SeenOrAdd(email) {
			return nil
		}
		collected = append(collected, email)
		return nil
	}); err != nil && err != r.Context().Err() {
		WriteError(w, http.StatusInternalServerError, "READ_ERROR", err.Error())
		return
	}

	job, err := h.mgr.Create(r.Context(), jobName, "upload_csv", int64(len(collected)), map[string]any{
		"filename": header.Filename,
		"size":     header.Size,
	})
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	if err := h.mgr.Submit(r.Context(), job.ID, collected); err != nil {
		WriteError(w, http.StatusInternalServerError, "SUBMIT_ERROR", err.Error())
		return
	}

	WriteJSON(w, http.StatusAccepted, map[string]any{
		"job_id": job.ID,
		"status": "queued",
		"total":  len(collected),
	})
}
