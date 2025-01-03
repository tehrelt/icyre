package suite

import (
	"testing"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/stretchr/testify/require"
)

type Config struct {
	Address string `env:"ADDRESS"`
	Timeout int    `env:"TIMEOUT"`
}

func NewConfig(t *testing.T) *Config {
	t.Helper()

	var cfg Config

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		require.NoError(t, err)
	}

	return &cfg
}
