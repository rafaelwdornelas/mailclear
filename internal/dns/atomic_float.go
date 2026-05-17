package dns

import "math"

// Wrappers para math.Float64bits/FromBits — só pra manter o arquivo
// adaptive.go limpo (sem imports de math diretamente lá).
func uint64Bits(f float64) uint64       { return math.Float64bits(f) }
func float64FromBits(b uint64) float64  { return math.Float64frombits(b) }
