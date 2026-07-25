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

	SupabaseURL            string
	SupabaseServiceRoleKey string
	SupabaseStorageBucket  string
}

// LoadAPI carga la configuración de cmd/api. Necesita Storage porque sirve
// los XML/PDF leyéndolos del bucket, no de la base.
func LoadAPI() (*Config, error) {
	cfg := load()
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("falta DATABASE_URL")
	}
	if err := requireStorage(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadWorker carga la configuración de cmd/worker, que además necesita IMAP
// para conectarse al buzón de ingesta, y Storage para subir los archivos.
func LoadWorker() (*Config, error) {
	cfg := load()
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("falta DATABASE_URL")
	}
	if cfg.ImapHost == "" || cfg.ImapUser == "" || cfg.ImapPassword == "" {
		return nil, fmt.Errorf("faltan credenciales IMAP (IMAP_HOST/IMAP_USER/IMAP_PASSWORD)")
	}
	if err := requireStorage(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func requireStorage(cfg *Config) error {
	if cfg.SupabaseURL == "" || cfg.SupabaseServiceRoleKey == "" || cfg.SupabaseStorageBucket == "" {
		return fmt.Errorf("faltan credenciales de Supabase Storage (SUPABASE_URL/SUPABASE_SERVICE_ROLE_KEY/SUPABASE_STORAGE_BUCKET)")
	}
	return nil
}

func load() *Config {
	return &Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		ImapHost:     os.Getenv("IMAP_HOST"),
		ImapPort:     envOr("IMAP_PORT", "993"),
		ImapUser:     os.Getenv("IMAP_USER"),
		ImapPassword: os.Getenv("IMAP_PASSWORD"),

		SupabaseURL:            os.Getenv("SUPABASE_URL"),
		SupabaseServiceRoleKey: os.Getenv("SUPABASE_SERVICE_ROLE_KEY"),
		SupabaseStorageBucket:  os.Getenv("SUPABASE_STORAGE_BUCKET"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
