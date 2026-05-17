// Binário mailclear-worker: roda apenas o worker pool (sem HTTP).
// PROJETO HARDCODED: sem flags, sem .env.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/coreos/go-systemd/v22/daemon"

	"github.com/rafaelwdornelas/mailclear/internal/app"
	"github.com/rafaelwdornelas/mailclear/internal/config"
	"github.com/rafaelwdornelas/mailclear/internal/shutdown"
)

func main() {
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-v" {
			fmt.Println("mailclear-worker dev")
			return
		}
	}

	cfg := config.Load()

	ctx, stop := shutdown.NotifyContext(context.Background())
	defer stop()

	a, err := app.New(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init app: %v\n", err)
		os.Exit(1)
	}
	defer a.Shutdown()

	a.Pool.Start(ctx)
	go a.AIMDCtrl.Run(ctx) // adaptive DNS concurrency

	if cfg.Disposable.UpdateEnabled {
		go a.DispUpdt.Run(ctx)
	}

	_, _ = daemon.SdNotify(false, daemon.SdNotifyReady)
	a.Log.Info().Int("workers", cfg.Workers.Count).Msg("worker process pronto")

	<-ctx.Done()
	a.Log.Info().Msg("sinal recebido, encerrando worker")

	_, _ = daemon.SdNotify(false, daemon.SdNotifyStopping)
	time.Sleep(500 * time.Millisecond)
}
