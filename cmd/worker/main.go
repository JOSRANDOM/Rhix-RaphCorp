// El worker de ingesta: se conecta por IMAP, procesa los correos UNSEEN con
// adjuntos XML, persiste cada Recibo por Honorarios y marca el correo como SEEN.
// Pensado para correr como Railway Cron Job: un proceso corto que termina solo.
package main

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"rhix-backend/internal/config"
	"rhix-backend/internal/lock"
)

func main() {
	_ = godotenv.Load() // no-op en Railway; útil en local si existe .env

	cfg, err := config.LoadWorker()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("conectando a Postgres: %v", err)
	}
	defer conn.Close(ctx)

	acquired, err := lock.TryAcquire(ctx, conn, lock.KeyInboundWorker)
	if err != nil {
		log.Fatalf("intentando tomar el lock: %v", err)
	}
	if !acquired {
		log.Println("otra corrida ya está procesando, se salta este ciclo")
		return
	}
	defer lock.Release(ctx, conn, lock.KeyInboundWorker)

	log.Println("lock adquirido, iniciando procesamiento de correos...")

	// TODO Fase 2: conectar por IMAP (internal/imap), listar UNSEEN.
	// TODO Fase 3: por cada adjunto .xml, parsear (internal/parser) y persistir (internal/receipt).
	// TODO: marcar el correo como SEEN solo después de persistir con éxito.

	log.Println("ciclo de procesamiento terminado")
}
