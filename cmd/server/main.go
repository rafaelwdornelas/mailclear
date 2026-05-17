// Binário mailclear-api: HTTP server + worker pool embarcado.
//
// PROJETO HARDCODED: sem flags, sem .env, sem config.yaml.
// Toda a configuração está em internal/config/defaults.go.
// Porta sempre 8181.
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

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-v" {
			fmt.Printf("mailclear-api %s (commit=%s build=%s)\n", version, commit, buildTime)
			return
		}
		if a == "--help" || a == "-h" {
			fmt.Println("mailclear-api — HTTP API + worker pool")
			fmt.Println("Sem flags. Tudo hardcoded em internal/config/defaults.go.")
			fmt.Println("Porta: 8181")
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

	if cfg.Disposable.UpdateEnabled {
		go a.DispUpdt.Run(ctx)
	}

	// Controle adaptativo da concorrência DNS (AIMD).
	go a.AIMDCtrl.Run(ctx)

	if _, err := daemon.SdNotify(false, daemon.SdNotifyReady); err != nil {
		a.Log.Debug().Err(err).Msg("sd_notify ready (ignorado fora do systemd)")
	}

	if interval, err := daemon.SdWatchdogEnabled(false); err == nil && interval > 0 {
		go watchdog(ctx, interval)
	}

	go func() {
		<-ctx.Done()
		_, _ = daemon.SdNotify(false, daemon.SdNotifyStopping)
	}()

	if err := a.HTTPServer.Start(); err != nil {
		a.Log.Fatal().Err(err).Msg("http server falhou")
	}
}

func watchdog(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval / 2)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = daemon.SdNotify(false, daemon.SdNotifyWatchdog)
		}
	}
}
