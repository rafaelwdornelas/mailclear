package handlers

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/rafaelwdornelas/mailclear/internal/storage"
)

// AdminOpsHandlers expõe operações que tradicionalmente exigem SSH+sudo:
// restart de serviço, leitura de journal, e destravar jobs.
//
// Restart usa D-Bus diretamente (pure-Go via go-systemd/v22/dbus). Sem polkit
// rule autorizando, a chamada falha com "Interactive authentication required."
// Ver deploy/polkit/10-mailclear.rules.
//
// Logs shellam para journalctl (em vez de sdjournal, que precisa CGO +
// libsystemd-dev no build). User precisa estar no grupo systemd-journal.
type AdminOpsHandlers struct {
	JobsRepo *storage.JobsRepo
	SelfUnit string // unit name do próprio processo (para tratamento especial no restart)
}

// NewAdminOps cria os handlers com a unit do próprio processo.
// selfUnit é tipicamente "mailclear-api.service" — passar vazio desabilita o
// tratamento de self-restart.
func NewAdminOps(jobs *storage.JobsRepo, selfUnit string) *AdminOpsHandlers {
	return &AdminOpsHandlers{JobsRepo: jobs, SelfUnit: selfUnit}
}

// RestartService reinicia mailclear-api ou mailclear-worker via D-Bus.
//   POST /admin/services/restart?service=api|worker
func (h *AdminOpsHandlers) RestartService(w http.ResponseWriter, r *http.Request) {
	unit, ok := unitFor(r.URL.Query().Get("service"))
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_SERVICE", "service deve ser 'api' ou 'worker'")
		return
	}

	// Self-restart precisa responder antes do shutdown derrubar o socket.
	if unit == h.SelfUnit {
		WriteJSON(w, http.StatusAccepted, map[string]any{
			"unit":      unit,
			"scheduled": "500ms",
			"note":      "API vai reiniciar — re-conecte em poucos segundos",
		})
		go h.deferredRestart(unit)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "DBUS_CONNECT", "dbus connect: "+err.Error())
		return
	}
	defer conn.Close()

	result, err := restartUnit(ctx, conn, unit)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "RESTART_FAILED", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"unit": unit, "result": result})
}

// deferredRestart roda assíncrono após a resposta sair. Usa context novo
// (não o do request, que termina quando o handler retorna).
func (h *AdminOpsHandlers) deferredRestart(unit string) {
	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		log.Error().Err(err).Str("unit", unit).Msg("self-restart: dbus connect falhou")
		return
	}
	defer conn.Close()

	if _, err := restartUnit(ctx, conn, unit); err != nil {
		log.Error().Err(err).Str("unit", unit).Msg("self-restart: RestartUnit falhou")
	}
}

// restartUnit dispara o restart e bloqueia até o systemd responder.
// O job result via channel é "done"/"canceled"/"timeout"/"failed"/"dependency"/"skipped".
func restartUnit(ctx context.Context, conn *dbus.Conn, unit string) (string, error) {
	ch := make(chan string, 1)
	if _, err := conn.RestartUnitContext(ctx, unit, "replace", ch); err != nil {
		return "", err
	}
	select {
	case res := <-ch:
		return res, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Logs devolve as últimas N linhas do journal da unit (texto plano).
//   GET /admin/logs?service=api|worker&lines=200
func (h *AdminOpsHandlers) Logs(w http.ResponseWriter, r *http.Request) {
	unit, ok := unitFor(r.URL.Query().Get("service"))
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_SERVICE", "service deve ser 'api' ou 'worker'")
		return
	}
	lines := 200
	if n, err := strconv.Atoi(r.URL.Query().Get("lines")); err == nil && n > 0 && n <= 5000 {
		lines = n
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "journalctl", "-u", unit,
		"-n", strconv.Itoa(lines), "--no-pager", "-o", "cat")
	out, err := cmd.CombinedOutput()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "JOURNALCTL_ERROR",
			"journalctl falhou (user precisa estar no grupo systemd-journal): "+string(out))
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

// LogsStream faz tail -f no journal via SSE.
//   GET /admin/logs/stream?service=api|worker
//
// Cada linha do journalctl vira um evento "data: <linha>\n\n".
// Cliente fecha via EventSource.close().
func (h *AdminOpsHandlers) LogsStream(w http.ResponseWriter, r *http.Request) {
	unit, ok := unitFor(r.URL.Query().Get("service"))
	if !ok {
		WriteError(w, http.StatusBadRequest, "BAD_SERVICE", "service deve ser 'api' ou 'worker'")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteError(w, http.StatusInternalServerError, "NO_FLUSH", "streaming não suportado")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // desliga buffering em proxies

	cmd := exec.CommandContext(r.Context(), "journalctl",
		"-u", unit, "-n", "50", "-f", "--no-pager", "-o", "cat")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "PIPE_ERROR", err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		WriteError(w, http.StatusInternalServerError, "JOURNAL_START", err.Error())
		return
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		if _, err := io.WriteString(w, "data: "+scanner.Text()+"\n\n"); err != nil {
			return
		}
		flusher.Flush()
	}
}

// ForceFail marca um job como failed independente do estado atual. Útil quando
// um job ficou em running por longo tempo sem progresso (não precisa esperar
// o timeout natural de 24h em waitForCompletion).
//   POST /admin/jobs/{id}/force-fail
func (h *AdminOpsHandlers) ForceFail(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_ID", "id inválido: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.JobsRepo.ForceFail(ctx, id, "manual force-fail via dashboard"); err != nil {
		WriteError(w, http.StatusUnprocessableEntity, "FORCE_FAIL_ERROR", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"id": id, "status": "failed"})
}

func unitFor(svc string) (string, bool) {
	switch svc {
	case "api":
		return "mailclear-api.service", true
	case "worker":
		return "mailclear-worker.service", true
	}
	return "", false
}
