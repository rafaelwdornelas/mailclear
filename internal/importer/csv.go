// Package importer lê CSVs massivos em streaming, extrai emails e dispara
// batches para o worker pool.
package importer

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strings"
)

// Reader lê linhas do CSV em streaming, sem carregar tudo na memória.
// A primeira coluna que parecer ser email é extraída — se não houver header,
// usa a coluna 0.
type Reader struct {
	r              *csv.Reader
	emailColumn    int  // -1 = auto-detect na primeira linha
	skippedHeader  bool
}

// NewReader cria um reader. delim define o separador (',' por padrão).
// Se delim==0, '/' é detectado heuristicamente — para o caso comum, usar ','.
func NewReader(src io.Reader, delim rune) *Reader {
	cr := csv.NewReader(src)
	if delim != 0 {
		cr.Comma = delim
	}
	cr.FieldsPerRecord = -1  // tolera variação
	cr.LazyQuotes = true
	return &Reader{r: cr, emailColumn: -1}
}

// Each chama fn para cada email encontrado. Para abortar, devolva um erro.
// Linhas vazias são puladas. Erros de CSV individuais são logados como skip.
func (r *Reader) Each(ctx context.Context, fn func(email string) error) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		row, err := r.r.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			// Linha inválida — pula
			continue
		}
		if len(row) == 0 {
			continue
		}

		if r.emailColumn < 0 {
			r.emailColumn = pickEmailColumn(row)
			// Se a primeira linha é claramente um header, pula
			if isHeader(row) {
				r.skippedHeader = true
				continue
			}
		}

		col := r.emailColumn
		if col >= len(row) {
			col = 0
		}
		email := strings.TrimSpace(row[col])
		if email == "" {
			continue
		}
		if err := fn(email); err != nil {
			return err
		}
	}
}

// pickEmailColumn devolve o índice da coluna que contém '@' ou 0.
func pickEmailColumn(row []string) int {
	for i, v := range row {
		if strings.Contains(v, "@") {
			return i
		}
	}
	return 0
}

// isHeader detecta linhas que parecem cabeçalho (sem '@' e com palavras comuns).
func isHeader(row []string) bool {
	for _, v := range row {
		lv := strings.ToLower(strings.TrimSpace(v))
		if lv == "email" || lv == "e-mail" || lv == "mail" || lv == "address" {
			return true
		}
	}
	return false
}
