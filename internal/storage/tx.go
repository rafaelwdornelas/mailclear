package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// TxRunner executa fn dentro de uma transação. Commita se fn não retornar erro,
// faz rollback caso contrário. Propaga o context para que cancelations interrompam.
func TxRunner(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		// pgx.ErrTxClosed é esperado após Commit. Outros erros (conexão caiu
		// no meio, ctx cancelou) são visíveis no log mas não propagam — o
		// caller já decidiu pelo resultado do Commit/fn.
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			log.Warn().Err(rbErr).Msg("tx rollback falhou")
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
