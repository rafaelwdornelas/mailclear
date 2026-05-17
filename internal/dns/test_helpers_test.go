package dns

import (
	"io"

	"github.com/rs/zerolog"
)

func noopLogger() zerolog.Logger {
	return zerolog.New(io.Discard)
}
