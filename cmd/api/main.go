// API delgada que sirve al frontend de Rhix: listado de recibos procesados y
// disparo de reportes. Corre como servicio web persistente en Railway (a
// diferencia del worker, que es un Cron Job de corta duración).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"

	"rhix-backend/internal/config"
	"rhix-backend/internal/db"
	"rhix-backend/internal/excel"
	"rhix-backend/internal/receipt"
	"rhix-backend/internal/storage"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.LoadAPI()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.ConnectPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	repo := receipt.NewRepository(pool)
	storageClient := storage.New(cfg)

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

	mux.HandleFunc("GET /api/receipts/{id}/xml", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "id inválido", http.StatusBadRequest)
			return
		}

		loc, err := repo.GetXMLLocation(r.Context(), id)
		if errors.Is(err, receipt.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var content []byte
		if loc.StoragePath != "" {
			content, err = storageClient.Download(r.Context(), loc.StoragePath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else if loc.RawXML != nil {
			content = []byte(*loc.RawXML)
		} else {
			http.Error(w, "este recibo no tiene XML guardado", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="recibo-%d.xml"`, id))
		w.Write(content)
	})

	mux.HandleFunc("GET /api/receipts/{id}/pdf", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "id inválido", http.StatusBadRequest)
			return
		}

		loc, err := repo.GetPDFLocation(r.Context(), id)
		if errors.Is(err, receipt.ErrNotFound) {
			http.Error(w, "este recibo no tiene PDF", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var content []byte
		if loc.StoragePath != "" {
			content, err = storageClient.Download(r.Context(), loc.StoragePath)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			content = loc.RawPDF
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="recibo-%d.pdf"`, id))
		w.Write(content)
	})

	mux.HandleFunc("GET /api/receipts/{id}/email", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "id inválido", http.StatusBadRequest)
			return
		}

		email, err := repo.GetEmail(r.Context(), id)
		if errors.Is(err, receipt.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(email)
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
