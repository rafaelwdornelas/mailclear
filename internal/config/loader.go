package config

// Load devolve a configuração hardcoded.
//
// Mantida com esta assinatura para compatibilidade com o resto da
// aplicação. Não lê arquivo nem variável de ambiente — todos os valores
// estão em defaults.go.
//
// Para tunar, edite defaults.go e recompile.
func Load() *Config {
	return Defaults()
}
