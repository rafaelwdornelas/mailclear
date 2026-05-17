// Package config define a configuração da aplicação.
//
// PROJETO HARDCODED: todos os valores são fixos no código, definidos em
// defaults.go. Não há leitura de .env, config.yaml ou variáveis de ambiente.
// A intenção é zero-config: instalou → roda → funciona.
//
// Para tuning, edite defaults.go e recompile.
package config

import "time"

// Config agrega todas as seções de configuração.
type Config struct {
	HTTP       HTTPConfig
	Log        LogConfig
	DB         DBConfig
	Redis      RedisConfig
	DNS        DNSConfig
	Workers    WorkersConfig
	RateLimit  RateLimitConfig
	Cache      CacheConfig
	Score      ScoreConfig
	CatchAll   CatchAllConfig
	Disposable DisposableConfig
}

// HTTPConfig — porta sempre 8181.
type HTTPConfig struct {
	Addr               string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	RateLimitPerMinute int
}

// LogConfig — JSON em info por padrão (compatível journalctl).
type LogConfig struct {
	Level  string
	Format string
}

// DBConfig — PostgreSQL local com user/senha "mailclear/mailclear".
type DBConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

// RedisConfig — Redis local sem auth.
type RedisConfig struct {
	URL       string
	PoolSize  int
	KeyPrefix string
}

// DNSConfig — Unbound local em 127.0.0.1:53.
type DNSConfig struct {
	Server     string
	Timeout    time.Duration
	Retries    int
	EDNSBuffer uint16
}

// WorkersConfig — auto-tune por NumCPU, paralelismo intra-batch.
type WorkersConfig struct {
	Count            int
	BatchSize        int
	JobQueueBuffer   int
	EmailConcurrency int
}

// RateLimitConfig.
type RateLimitConfig struct {
	PerDomain int
}

// CacheConfig.
type CacheConfig struct {
	LRUSize     int
	TTLMin      time.Duration
	TTLMax      time.Duration
	TTLNegative time.Duration
}

// ScoreConfig pesos do score engine + thresholds.
type ScoreConfig struct {
	ValidMin int
	RiskyMin int
	Weights  ScoreWeights
}

// ScoreWeights deltas aplicados a cada sinal.
type ScoreWeights struct {
	Syntax     int
	TLD        int
	MX         int
	AFallback  int
	Disposable int
	Role       int
	Typo       int
	CatchAll   int
}

// CatchAllConfig habilita probe SMTP (off por padrão).
type CatchAllConfig struct {
	ProbeEnabled bool
}

// DisposableConfig — pull periódico da lista pública.
type DisposableConfig struct {
	UpdateEnabled  bool
	UpdateInterval time.Duration
	SourceURL      string
}
