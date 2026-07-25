// Package storage sube y descarga archivos del bucket de Supabase Storage
// (los XML/PDF de los recibos), hablando directo con su API REST — no hace
// falta un SDK para subir/bajar/borrar un objeto.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"rhix-backend/internal/config"
)

// ErrNotFound indica que el objeto no existe en el bucket.
var ErrNotFound = errors.New("archivo no encontrado en storage")

// Client sube y descarga objetos de un bucket de Supabase Storage.
type Client struct {
	baseURL    string
	bucket     string
	serviceKey string
	httpClient *http.Client
}

func New(cfg *config.Config) *Client {
	return &Client{
		baseURL:    cfg.SupabaseURL,
		bucket:     cfg.SupabaseStorageBucket,
		serviceKey: cfg.SupabaseServiceRoleKey,
		httpClient: http.DefaultClient,
	}
}

// Upload sube el contenido a la ruta indicada dentro del bucket (la
// sobreescribe si ya existía). path es relativo al bucket, p. ej.
// "10012345678/E001-123.xml".
func (c *Client) Upload(ctx context.Context, path string, content []byte, contentType string) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", c.baseURL, c.bucket, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("armando request de subida: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	req.Header.Set("apikey", c.serviceKey)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-upsert", "true")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("subiendo %q: %w", path, err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("subiendo %q: status %d: %s", path, res.StatusCode, string(body))
	}
	return nil
}

// Download trae el contenido de un objeto del bucket.
func (c *Client) Download(ctx context.Context, path string) ([]byte, error) {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", c.baseURL, c.bucket, path)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("armando request de descarga: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.serviceKey)
	req.Header.Set("apikey", c.serviceKey)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("descargando %q: %w", path, err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("descargando %q: status %d: %s", path, res.StatusCode, string(body))
	}

	return io.ReadAll(res.Body)
}
