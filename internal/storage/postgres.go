// Package storage encapsula a conexão com PostgreSQL via pgxpool.
package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rafaelwdornelas/mailclear/internal/config"
)

// NewPool inicializa um pgxpool.Pool a partir da config e valida com Ping.
func NewPool(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	pgCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse db url: %w", err)
	}
	if cfg.MaxConns > 0 {
		pgCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		pgCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		pgCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}

	pool, err := pgxpool.NewWithConfig(ctx, pgCfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}
