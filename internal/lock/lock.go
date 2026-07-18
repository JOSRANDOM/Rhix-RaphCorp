// Package lock implementa el candado anti-solapamiento del worker usando
// pg_try_advisory_lock de Postgres. Cada corrida del Cron Job de Railway es un
// contenedor nuevo sin estado compartido en memoria, así que el lock vive en la
// conexión a Postgres: si otra corrida ya lo tiene, TryAcquire retorna false de
// inmediato (no bloquea) y el worker simplemente se salta ese ciclo. Si el proceso
// muere a medio camino, Postgres libera el lock solo al cerrarse la conexión.
package lock

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// KeyInboundWorker identifica el lock del worker de ingesta de correos.
// Cualquier int64 arbitrario sirve, pero debe ser estable en el tiempo.
const KeyInboundWorker int64 = 727100

// TryAcquire intenta tomar el advisory lock. Devuelve false si otra corrida ya lo tiene.
func TryAcquire(ctx context.Context, conn *pgx.Conn, key int64) (bool, error) {
	var acquired bool
	err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired)
	if err != nil {
		return false, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	return acquired, nil
}

// Release libera el advisory lock explícitamente (además de liberarse solo al cerrar la conexión).
func Release(ctx context.Context, conn *pgx.Conn, key int64) error {
	_, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", key)
	if err != nil {
		return fmt.Errorf("pg_advisory_unlock: %w", err)
	}
	return nil
}
