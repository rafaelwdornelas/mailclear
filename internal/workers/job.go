// Package workers implementa um worker pool sobre channels com backpressure,
// retry com backoff e shutdown gracioso.
package workers

import (
	"context"

	"github.com/google/uuid"
)

// Task é a unidade de trabalho processada por um worker.
// Each task tem um job_id associado para tracking de progresso/persistência.
type Task struct {
	JobID  uuid.UUID
	Emails []string // batch a validar (pode ser len=1 para single)
}

// TaskHandler é a função executada por cada worker para processar uma task.
type TaskHandler func(ctx context.Context, t *Task) error
