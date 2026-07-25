package config

import (
	"fmt"
	"os"
)

// Config agrupa las variables de entorno que el worker y la API necesitan.
type Config struct {
	DatabaseURL string

	ImapHost     string
	ImapPort     string
	ImapUser     string
	ImapPassword string
}

// LoadAPI carga la configuración de cmd/api, que solo lee/expone datos ya
// persistidos y por lo tanto no necesita credenciales IMAP.
func LoadAPI() (*Config, error) {
	cfg := load()
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("falta DATABASE_URL")
	}
	return cfg, nil
}

// LoadWorker carga la configuración de cmd/worker, que además necesita IMAP
// para conectarse al buzón de ingesta.
func LoadWorker() (*Config, error) {
	cfg := load()
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("falta DATABASE_URL")
	}
	if cfg.ImapHost == "" || cfg.ImapUser == "" || cfg.ImapPassword == "" {
		return nil, fmt.Errorf("faltan credenciales IMAP (IMAP_HOST/IMAP_USER/IMAP_PASSWORD)")
	}
	return cfg, nil
}

func load() *Config {
	return &Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		ImapHost:     os.Getenv("IMAP_HOST"),
		ImapPort:     envOr("IMAP_PORT", "993"),
		ImapUser:     os.Getenv("IMAP_USER"),
		ImapPassword: os.Getenv("IMAP_PASSWORD"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
