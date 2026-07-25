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

	"github.com/joho/godotenv"

	"rhix-backend/internal/config"
	"rhix-backend/internal/db"
	"rhix-backend/internal/excel"
	"rhix-backend/internal/receipt"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.LoadAPI()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	conn, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
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

	mux.HandleFunc("GET /api/receipts/export", func(w http.ResponseWriter, r *http.Request) {
		receipts, err := repo.List(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		xlsxBytes, err := excel.GenerateReceiptsXLSX(receipts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="recibos.xlsx"`)
		w.Write(xlsxBytes)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("API escuchando en :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, withCORS(mux)))
}

// withCORS permite que el frontend (otro origen) consuma esta API. No hay
// cookies ni credenciales involucradas — todo lo que expone esta API es
// lectura pública de recibos ya procesados.
func withCORS(next http.Handler) http.Handler {
	origin := os.Getenv("CORS_ORIGIN")
	if origin == "" {
		origin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		next.ServeHTTP(w, r)
	})
}
