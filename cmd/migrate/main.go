// Binário mailclear-migrate: wrapper de golang-migrate.
// PROJETO HARDCODED: DB URL fixo (config.Defaults), sem .env.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/rafaelwdornelas/mailclear/internal/config"
)

func main() {
	migrationsDir := flag.String("dir", "migrations", "diretório com as migrations")
	flag.Parse()

	url := config.Defaults().DB.URL

	cmd := "up"
	args := flag.Args()
	if len(args) > 0 {
		cmd = args[0]
	}

	absDir, err := filepath.Abs(*migrationsDir)
	if err != nil {
		exitErr("resolver migrations dir: %v", err)
	}

	src := "file://" + absDir
	m, err := migrate.New(src, url)
	if err != nil {
		exitErr("init migrate: %v", err)
	}
	defer m.Close()

	switch cmd {
	case "up":
		err = m.Up()
	case "down":
		n := 1
		if len(args) > 1 {
			fmt.Sscanf(args[1], "%d", &n)
		}
		err = m.Steps(-n)
	case "drop":
		err = m.Drop()
	case "force":
		if len(args) < 2 {
			exitErr("uso: migrate force <version>")
		}
		var v int
		fmt.Sscanf(args[1], "%d", &v)
		err = m.Force(v)
	case "version":
		v, dirty, verr := m.Version()
		if verr != nil && !errors.Is(verr, migrate.ErrNilVersion) {
			exitErr("version: %v", verr)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return
	default:
		exitErr("comando desconhecido: %s (use: up|down|drop|force|version)", cmd)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		exitErr("migrate %s: %v", cmd, err)
	}
	fmt.Printf("migrate %s: ok\n", cmd)
}

func exitErr(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
