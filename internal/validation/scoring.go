package validation

import "github.com/rafaelwdornelas/mailclear/internal/config"

// ScoreEngine combina sinais coletados pelo pipeline em um score 0-100
// e classifica o email em valid|risky|invalid.
type ScoreEngine struct {
	cfg config.ScoreConfig
}

// NewScoreEngine cria um engine.
func NewScoreEngine(cfg config.ScoreConfig) *ScoreEngine {
	return &ScoreEngine{cfg: cfg}
}

// Apply consome o estado do *Email e seta Score + Status final.
// Se o pipeline já deu short-circuit (Status preenchido como invalid), respeita.
func (e *ScoreEngine) Apply(em *Email) {
	final := em.Status

	w := e.cfg.Weights
	score := 0

	if em.Normalized != "" {
		score += w.Syntax
	}
	if em.HasMX {
		score += w.MX
	} else if em.HasA {
		score += w.AFallback
	}
	score += w.TLD

	if em.IsDisposable {
		score += w.Disposable
	}
	if em.IsRole {
		score += w.Role
	}
	if em.Suggested != "" {
		score += w.Typo
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	em.Score = score

	if final == "" {
		switch {
		case em.IsDisposable:
			em.Status = StatusDisposable
		case score >= e.cfg.ValidMin:
			em.Status = StatusValid
		case score >= e.cfg.RiskyMin:
			em.Status = StatusRisky
		default:
			em.Status = StatusInvalid
		}
	}
}
