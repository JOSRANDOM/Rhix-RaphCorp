// API delgada que sirve al frontend de Rhix: listado de recibos procesados y
// disparo de reportes. Corre como servicio web persistente en Railway (a
// diferencia del worker, que es un Cron Job de corta duración).
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"

	"rhix-backend/internal/config"
	"rhix-backend/internal/receipt"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.LoadAPI()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("conectando a Postgres: %v", err)
	}
	defer conn.Close(ctx)

	repo := receipt.NewRepository(conn)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /api/receipts", func(w http.ResponseWriter, r *http.Request) {
		receipts, err := repo.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(receipts)
	})

	// TODO Fase 4: GET /api/receipts/export -> genera el xlsx con internal/excel.

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("API escuchando en :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
