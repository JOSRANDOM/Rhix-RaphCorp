// Package db centraliza la conexión a Postgres.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Connect abre una conexión usando el protocolo simple de consultas (sin
// sentencias preparadas nombradas). Supabase se conecta vía el Transaction
// Pooler (PgBouncer en modo transacción), que multiplexa cada conexión de
// cliente sobre distintas conexiones reales a Postgres; el modo por defecto
// de pgx prepara sentencias con un nombre determinístico, y esas sentencias
// quedan "pegadas" a la conexión real de Postgres, no a la del cliente — la
// siguiente corrida puede toparse con una sentencia del mismo nombre ya
// preparada por otra conexión y fallar con "prepared statement already
// exists". El protocolo simple evita sentencias preparadas por completo.
func Connect(ctx context.Context, databaseURL string) (*pgx.Conn, error) {
	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parseando DATABASE_URL: %w", err)
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("conectando a Postgres: %w", err)
	}
	return conn, nil
}
