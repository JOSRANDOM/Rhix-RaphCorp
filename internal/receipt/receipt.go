// Package receipt contiene el modelo de dominio y el acceso a datos de los
// Recibos por Honorarios procesados.
package receipt

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound indica que no existe un recibo (o un archivo suyo) con el ID solicitado.
var ErrNotFound = errors.New("recibo no encontrado")

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
	XMLStoragePath string    `json:"-"`
	PDFStoragePath string    `json:"-"`
	HasPDF         bool      `json:"hasPdf"`
	Status         Status    `json:"status"`
	ErrorMessage   *string   `json:"errorMessage,omitempty"`
	EmailMessageID string    `json:"emailMessageId"`
	// Datos del correo original — no van en el listado (se piden aparte con
	// GetEmail, ver EmailDetail), solo se usan al insertar.
	EmailFrom    string    `json:"-"`
	EmailTo      string    `json:"-"`
	EmailCc      string    `json:"-"`
	EmailSubject string    `json:"-"`
	EmailBody    string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

// EmailDetail son los datos del correo original de un recibo, para mostrarlo
// completo en el frontend (de quién viene, para quién, copia, asunto, cuerpo,
// y los archivos que trajo — un correo puede traer más de un adjunto .xml,
// y cada uno queda como su propia fila de receipts con el mismo EmailMessageID).
type EmailDetail struct {
	From        string            `json:"from"`
	To          string            `json:"to"`
	Cc          *string           `json:"cc,omitempty"`
	Subject     string            `json:"subject"`
	Body        string            `json:"body"`
	Attachments []EmailAttachment `json:"attachments"`
}

// EmailAttachment identifica un recibo/adjunto que llegó en el mismo correo.
type EmailAttachment struct {
	ID          int64  `json:"id"`
	SerieNumero string `json:"serieNumero"`
	HasPDF      bool   `json:"hasPdf"`
}

// XMLLocation indica dónde está el XML de un recibo: en el bucket de Storage
// (StoragePath no vacío, el caso normal desde que existe internal/storage) o
// guardado directo en la fila (RawXML, recibos persistidos antes de eso).
type XMLLocation struct {
	StoragePath string
	RawXML      *string
}

// PDFLocation es el equivalente de XMLLocation para el PDF. Puede no haber
// ninguno de los dos (el correo no traía PDF).
type PDFLocation struct {
	StoragePath string
	RawPDF      []byte
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
	var emailCc *string
	if rcpt.EmailCc != "" {
		emailCc = &rcpt.EmailCc
	}
	var xmlPath, pdfPath *string
	if rcpt.XMLStoragePath != "" {
		xmlPath = &rcpt.XMLStoragePath
	}
	if rcpt.PDFStoragePath != "" {
		pdfPath = &rcpt.PDFStoragePath
	}

	_, err := r.conn.Exec(ctx, `
		INSERT INTO receipts
			(ruc, razon_social, serie_numero, fecha_emision, monto_neto, retencion, status, error_message, email_message_id,
			 email_from, email_to, email_cc, email_subject, email_body, xml_storage_path, pdf_storage_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (ruc, serie_numero) DO NOTHING`,
		rcpt.RUC, rcpt.RazonSocial, rcpt.SerieNumero, rcpt.FechaEmision,
		rcpt.MontoNeto, rcpt.Retencion, rcpt.Status, rcpt.ErrorMessage, rcpt.EmailMessageID,
		rcpt.EmailFrom, rcpt.EmailTo, emailCc, rcpt.EmailSubject, rcpt.EmailBody, xmlPath, pdfPath,
	)
	return err
}

func (r *Repository) List(ctx context.Context) ([]Receipt, error) {
	rows, err := r.conn.Query(ctx, `
		SELECT id, ruc, razon_social, serie_numero, fecha_emision, monto_neto, retencion, status, email_message_id, created_at,
		       (raw_pdf IS NOT NULL OR pdf_storage_path IS NOT NULL) AS has_pdf
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
			&rcpt.EmailMessageID, &rcpt.CreatedAt, &rcpt.HasPDF); err != nil {
			return nil, err
		}
		out = append(out, rcpt)
	}
	return out, rows.Err()
}

// GetXMLLocation dice de dónde leer el XML de un recibo (bucket o columna vieja).
func (r *Repository) GetXMLLocation(ctx context.Context, id int64) (XMLLocation, error) {
	var loc XMLLocation
	var storagePath *string
	err := r.conn.QueryRow(ctx,
		`SELECT xml_storage_path, raw_xml FROM receipts WHERE id = $1`, id,
	).Scan(&storagePath, &loc.RawXML)
	if errors.Is(err, pgx.ErrNoRows) {
		return XMLLocation{}, ErrNotFound
	}
	if err != nil {
		return XMLLocation{}, err
	}
	if storagePath != nil {
		loc.StoragePath = *storagePath
	}
	return loc, nil
}

// GetPDFLocation dice de dónde leer el PDF de un recibo (bucket o columna
// vieja). Devuelve ErrNotFound si el recibo no tiene PDF en ningún lado.
func (r *Repository) GetPDFLocation(ctx context.Context, id int64) (PDFLocation, error) {
	var loc PDFLocation
	var storagePath *string
	err := r.conn.QueryRow(ctx,
		`SELECT pdf_storage_path, raw_pdf FROM receipts WHERE id = $1`, id,
	).Scan(&storagePath, &loc.RawPDF)
	if errors.Is(err, pgx.ErrNoRows) {
		return PDFLocation{}, ErrNotFound
	}
	if err != nil {
		return PDFLocation{}, err
	}
	if storagePath != nil {
		loc.StoragePath = *storagePath
	}
	if loc.StoragePath == "" && loc.RawPDF == nil {
		return PDFLocation{}, ErrNotFound
	}
	return loc, nil
}

// GetEmail devuelve los datos del correo original de un recibo, más la lista
// de todos los adjuntos que llegaron en ese mismo correo (mismo
// email_message_id) para poder descargarlos desde ahí. Los recibos
// persistidos antes de que existieran estas columnas devuelven campos vacíos.
func (r *Repository) GetEmail(ctx context.Context, id int64) (EmailDetail, error) {
	var detail EmailDetail
	var messageID string
	err := r.conn.QueryRow(ctx,
		`SELECT email_from, email_to, email_cc, email_subject, email_body, email_message_id
		 FROM receipts WHERE id = $1`,
		id,
	).Scan(&detail.From, &detail.To, &detail.Cc, &detail.Subject, &detail.Body, &messageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return EmailDetail{}, ErrNotFound
	}
	if err != nil {
		return EmailDetail{}, err
	}

	rows, err := r.conn.Query(ctx,
		`SELECT id, serie_numero, (raw_pdf IS NOT NULL OR pdf_storage_path IS NOT NULL) AS has_pdf
		 FROM receipts WHERE email_message_id = $1 ORDER BY id`,
		messageID,
	)
	if err != nil {
		return EmailDetail{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var att EmailAttachment
		if err := rows.Scan(&att.ID, &att.SerieNumero, &att.HasPDF); err != nil {
			return EmailDetail{}, err
		}
		detail.Attachments = append(detail.Attachments, att)
	}
	if err := rows.Err(); err != nil {
		return EmailDetail{}, err
	}

	return detail, nil
}
