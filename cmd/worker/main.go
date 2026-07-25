// El worker de ingesta: se conecta por IMAP, procesa los correos UNSEEN con
// adjuntos XML, persiste cada Recibo por Honorarios y marca el correo como SEEN.
// Pensado para correr como Railway Cron Job: un proceso corto que termina solo.
package main

import (
	"context"
	"log"
	"time"

	"github.com/joho/godotenv"

	"rhix-backend/internal/config"
	"rhix-backend/internal/db"
	"rhix-backend/internal/inbox"
	"rhix-backend/internal/lock"
	"rhix-backend/internal/parser"
	"rhix-backend/internal/receipt"
)

func main() {
	_ = godotenv.Load() // no-op en Railway; útil en local si existe .env

	cfg, err := config.LoadWorker()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	conn, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
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

	repo := receipt.NewRepository(conn)

	for _, email := range pending {
		if processEmail(ctx, repo, email) {
			if err := session.MarkSeen(email.UID); err != nil {
				log.Printf("UID %d: no se pudo marcar como leído: %v", email.UID, err)
			}
		}
	}

	log.Println("ciclo de procesamiento terminado")
}

// processEmail parsea y persiste cada adjunto .xml de un correo. Devuelve
// true solo si todos los adjuntos se persistieron con éxito (o ya existían),
// es decir, si el correo puede marcarse como SEEN sin perder nada por reintentar.
func processEmail(ctx context.Context, repo *receipt.Repository, email inbox.PendingEmail) bool {
	allOK := true

	// Si el correo trae exactamente un PDF, lo asociamos a cada recibo que
	// salga de este correo (el caso normal: un XML + su PDF del mismo
	// documento). Con cero o más de un PDF no adivinamos a cuál corresponde.
	var pdfContent []byte
	if len(email.PDFAttachments) == 1 {
		pdfContent = email.PDFAttachments[0].Content
	}

	for _, att := range email.Attachments {
		parsed, err := parser.Parse(att.Content)
		if err != nil {
			log.Printf("UID %d, adjunto %q: error al parsear: %v", email.UID, att.Filename, err)
			allOK = false
			continue
		}

		exists, err := repo.Exists(ctx, parsed.RUC, parsed.SerieNumero)
		if err != nil {
			log.Printf("UID %d, adjunto %q: error verificando duplicado: %v", email.UID, att.Filename, err)
			allOK = false
			continue
		}
		if exists {
			log.Printf("UID %d, adjunto %q: recibo %s-%s ya existía, se omite", email.UID, att.Filename, parsed.RUC, parsed.SerieNumero)
			continue
		}

		rcpt := receipt.Receipt{
			RUC:            parsed.RUC,
			RazonSocial:    parsed.RazonSocial,
			SerieNumero:    parsed.SerieNumero,
			FechaEmision:   parsed.FechaEmision,
			MontoNeto:      parsed.MontoNeto,
			Retencion:      parsed.Retencion,
			RawXML:         string(att.Content),
			Status:         receipt.StatusProcessed,
			EmailMessageID: email.MessageID,
			EmailFrom:      email.From,
			EmailTo:        email.To,
			EmailCc:        email.Cc,
			EmailSubject:   email.Subject,
			EmailBody:      email.Body,
			RawPDF:         pdfContent,
		}

		if err := repo.Create(ctx, rcpt); err != nil {
			log.Printf("UID %d, adjunto %q: error al persistir: %v", email.UID, att.Filename, err)
			allOK = false
			continue
		}

		log.Printf("UID %d, adjunto %q: recibo %s-%s persistido", email.UID, att.Filename, parsed.RUC, parsed.SerieNumero)
	}

	return allOK
}
