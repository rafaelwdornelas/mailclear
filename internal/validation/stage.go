package validation

import "context"

// Stage é uma etapa do pipeline.
// Run executa a lógica; deve mutar *Email diretamente.
// Devolve erro para sinalizar falha lógica (curto-circuitada se ShortCircuit=true).
type Stage interface {
	Name() string
	Run(ctx context.Context, e *Email) error
	ShortCircuit() bool
}
