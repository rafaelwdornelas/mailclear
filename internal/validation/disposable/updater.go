package disposable

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Updater faz pull periódico de uma URL pública (ex: GitHub raw) para refrescar
// a lista de disposable. É opcional — o Registry sempre tem a lista embedded como base.
type Updater struct {
	SourceURL string
	Registry  *Registry
	Interval  time.Duration
	HTTP      *http.Client
}

// NewUpdater cria um updater pronto para uso.
func NewUpdater(reg *Registry, url string, interval time.Duration) *Updater {
	return &Updater{
		SourceURL: url,
		Registry:  reg,
		Interval:  interval,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

// Run dispara um loop de atualização periódica. Bloqueante; rodar em goroutine.
// Cancela quando o ctx é cancelado.
func (u *Updater) Run(ctx context.Context) {
	t := time.NewTicker(u.Interval)
	defer t.Stop()

	// Primeira execução sem esperar
	if err := u.RunOnce(ctx); err != nil {
		// Apenas loga via Println — o caller injetará o logger se quiser
		fmt.Printf("disposable updater: primeira atualização falhou: %v\n", err)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := u.RunOnce(ctx); err != nil {
				fmt.Printf("disposable updater: %v\n", err)
			}
		}
	}
}

// RunOnce baixa a lista uma vez e atualiza o Registry.
func (u *Updater) RunOnce(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.SourceURL, nil)
	if err != nil {
		return err
	}
	resp, err := u.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8MB cap
	if err != nil {
		return err
	}
	u.Registry.LoadString(string(b))
	return nil
}
