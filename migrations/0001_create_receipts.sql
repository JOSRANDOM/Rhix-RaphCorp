CREATE TABLE IF NOT EXISTS receipts (
    id SERIAL PRIMARY KEY,
    ruc TEXT NOT NULL,
    razon_social TEXT NOT NULL,
    serie_numero TEXT NOT NULL,
    fecha_emision DATE NOT NULL,
    monto_neto NUMERIC(12,2) NOT NULL,
    retencion NUMERIC(12,2),
    raw_xml TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    email_message_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ruc, serie_numero)
);

CREATE INDEX IF NOT EXISTS idx_receipts_status ON receipts (status);
