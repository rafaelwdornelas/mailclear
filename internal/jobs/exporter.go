package jobs

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/google/uuid"

	"github.com/rafaelwdornelas/mailclear/internal/storage"
)

// ExportCSV escreve todos os resultados do job em formato CSV no writer.
// Faz streaming: pagina por cursor (id) para não carregar tudo em memória.
func ExportCSV(ctx context.Context, repo *storage.ResultsRepo, jobID uuid.UUID, w io.Writer, pageSize int32) error {
	if pageSize <= 0 {
		pageSize = 1000
	}

	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write([]string{
		"email_original", "email_normalized", "domain", "status", "classification",
		"score", "suggested_email", "is_disposable", "is_role", "has_mx",
	}); err != nil {
		return err
	}

	var afterID int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rows, err := repo.ListByJob(ctx, jobID, "", afterID, pageSize)
		if err != nil {
			return fmt.Errorf("paginar: %w", err)
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			suggested := ""
			if r.SuggestedEmail != nil {
				suggested = *r.SuggestedEmail
			}
			if err := cw.Write([]string{
				r.EmailOriginal,
				r.EmailNormalized,
				r.Domain,
				r.Status,
				r.Classification,
				strconv.Itoa(int(r.Score)),
				suggested,
				boolStr(r.IsDisposable),
				boolStr(r.IsRole),
				boolStr(r.HasMX),
			}); err != nil {
				return err
			}
			if r.ID > afterID {
				afterID = r.ID
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			return err
		}
	}
	return nil
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
