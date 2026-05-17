// Binário mailclear-seed-disposable: atualiza a lista de disposable_domains.
// PROJETO HARDCODED: sem flags de config.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rafaelwdornelas/mailclear/internal/config"
	"github.com/rafaelwdornelas/mailclear/internal/storage"
)

func main() {
	cfg := config.Load()
	url := cfg.Disposable.SourceURL

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := storage.NewPool(ctx, cfg.DB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := storage.NewDisposableRepo(db)

	fmt.Printf("Baixando lista: %s\n", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		exitErr("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		exitErr("download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		exitErr("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		exitErr("read body: %v", err)
	}

	var domains []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domains = append(domains, line)
	}
	fmt.Printf("Parseados: %d domínios\n", len(domains))

	const chunk = 5000
	for i := 0; i < len(domains); i += chunk {
		end := i + chunk
		if end > len(domains) {
			end = len(domains)
		}
		if err := repo.BulkUpsert(ctx, domains[i:end], "github:disposable-email-domains"); err != nil {
			exitErr("upsert: %v", err)
		}
	}
	count, _ := repo.Count(ctx)
	fmt.Printf("Total no banco: %d\n", count)
}

func exitErr(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
