// Package db centraliza la conexión a Postgres.
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier es lo mínimo que necesitan los repositorios para hacer consultas.
// Tanto *pgx.Conn (una sola conexión, la usa cmd/worker porque el advisory
// lock necesita quedarse en la misma sesión) como *pgxpool.Pool (varias
// conexiones con reconexión automática, la usa cmd/api) lo implementan.
type Querier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

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

// ConnectPool abre un pool de conexiones (mismo protocolo simple que
// Connect). A diferencia de una conexión única, el pool reemplaza solo las
// conexiones que se caen — una sentencia falla una vez, no deja rota la API
// entera hasta reiniciarla a mano. Para cmd/api, que es un servicio de larga
// duración; cmd/worker sigue usando Connect porque el advisory lock necesita
// una sola conexión dedicada.
func ConnectPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parseando DATABASE_URL: %w", err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creando el pool de Postgres: %w", err)
	}
	return pool, nil
}
