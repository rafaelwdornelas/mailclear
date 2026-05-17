// Package server gerencia o ciclo de vida do http.Server.
package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/rafaelwdornelas/mailclear/internal/config"
)

// Server agrupa o http.Server e um logger.
type Server struct {
	srv *http.Server
	log zerolog.Logger
}

// New cria um http.Server configurado.
func New(cfg config.HTTPConfig, handler http.Handler, log zerolog.Logger) *Server {
	return &Server{
		srv: &http.Server{
			Addr:         cfg.Addr,
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
		log: log.With().Str("component", "http_server").Logger(),
	}
}

// Start lança o listener. Bloqueante até erro fatal.
func (s *Server) Start() error {
	s.log.Info().Str("addr", s.srv.Addr).Msg("http server iniciado")
	err := s.srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown faz graceful shutdown com timeout.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	s.log.Info().Msg("http server: graceful shutdown iniciado")
	return s.srv.Shutdown(ctx)
}
