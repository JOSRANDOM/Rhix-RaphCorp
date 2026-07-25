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
	"rhix-backend/internal/inbox"
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

	session, err := inbox.Connect(cfg)
	if err != nil {
		log.Fatalf("conectando al buzón IMAP: %v", err)
	}
	defer session.Close()

	pending, err := session.FetchPendingXML()
	if err != nil {
		log.Fatalf("listando correos pendientes: %v", err)
	}

	log.Printf("%d correo(s) sin leer con adjuntos .xml", len(pending))
	for _, email := range pending {
		log.Printf("UID %d — %q (%d adjunto(s))", email.UID, email.Subject, len(email.Attachments))
	}

	// TODO Fase 3: por cada adjunto .xml, parsear (internal/parser) y persistir
	// (internal/receipt), y llamar a session.MarkSeen(email.UID) solo después
	// de persistir con éxito.

	log.Println("ciclo de procesamiento terminado")
}
