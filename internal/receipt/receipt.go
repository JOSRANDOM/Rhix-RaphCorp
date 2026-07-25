// Package receipt contiene el modelo de dominio y el acceso a datos de los
// Recibos por Honorarios procesados.
package receipt

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusProcessed Status = "processed"
	StatusFailed    Status = "failed"
)

type Receipt struct {
	ID             int64     `json:"id"`
	RUC            string    `json:"ruc"`
	RazonSocial    string    `json:"razonSocial"`
	SerieNumero    string    `json:"serieNumero"`
	FechaEmision   time.Time `json:"fechaEmision"`
	MontoNeto      float64   `json:"montoNeto"`
	Retencion      *float64  `json:"retencion,omitempty"`
	RawXML         string    `json:"-"`
	Status         Status    `json:"status"`
	ErrorMessage   *string   `json:"errorMessage,omitempty"`
	EmailMessageID string    `json:"emailMessageId"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Repository struct {
	conn *pgx.Conn
}

func NewRepository(conn *pgx.Conn) *Repository {
	return &Repository{conn: conn}
}

// Exists verifica duplicados por RUC + Serie (además del UNIQUE constraint en BD,
// que es la garantía real contra condiciones de carrera).
func (r *Repository) Exists(ctx context.Context, ruc, serieNumero string) (bool, error) {
	var exists bool
	err := r.conn.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM receipts WHERE ruc = $1 AND serie_numero = $2)`,
		ruc, serieNumero,
	).Scan(&exists)
	return exists, err
}

func (r *Repository) Create(ctx context.Context, rcpt Receipt) error {
	_, err := r.conn.Exec(ctx, `
		INSERT INTO receipts
			(ruc, razon_social, serie_numero, fecha_emision, monto_neto, retencion, raw_xml, status, error_message, email_message_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (ruc, serie_numero) DO NOTHING`,
		rcpt.RUC, rcpt.RazonSocial, rcpt.SerieNumero, rcpt.FechaEmision,
		rcpt.MontoNeto, rcpt.Retencion, rcpt.RawXML, rcpt.Status, rcpt.ErrorMessage, rcpt.EmailMessageID,
	)
	return err
}

func (r *Repository) List(ctx context.Context) ([]Receipt, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT id, ruc, razon_social, serie_numero, fecha_emision, monto_neto, retencion, status, email_message_id, created_at
		FROM receipts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Receipt
	for rows.Next() {
		var rcpt Receipt
		if err := rows.Scan(&rcpt.ID, &rcpt.RUC, &rcpt.RazonSocial, &rcpt.SerieNumero,
			&rcpt.FechaEmision, &rcpt.MontoNeto, &rcpt.Retencion, &rcpt.Status,
			&rcpt.EmailMessageID, &rcpt.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rcpt)
	}
	return out, rows.Err()
}
