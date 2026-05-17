package importer

// Batcher acumula emails em lotes e dispara flush quando atinge o tamanho alvo.
// Use Close para garantir o último batch.
type Batcher struct {
	size  int
	buf   []string
	flush func(batch []string) error
}

// NewBatcher cria um batcher.
func NewBatcher(size int, flush func(batch []string) error) *Batcher {
	if size <= 0 {
		size = 1000
	}
	return &Batcher{size: size, buf: make([]string, 0, size), flush: flush}
}

// Add adiciona um email. Pode disparar flush automaticamente.
func (b *Batcher) Add(email string) error {
	b.buf = append(b.buf, email)
	if len(b.buf) >= b.size {
		return b.Flush()
	}
	return nil
}

// Flush descarrega o batch atual.
func (b *Batcher) Flush() error {
	if len(b.buf) == 0 {
		return nil
	}
	batch := make([]string, len(b.buf))
	copy(batch, b.buf)
	b.buf = b.buf[:0]
	return b.flush(batch)
}

// Close garante que o último batch seja descarregado.
func (b *Batcher) Close() error {
	return b.Flush()
}
