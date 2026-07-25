-- Guarda el PDF original del recibo (cuando el correo trae uno), para poder
-- verlo/descargarlo igual que el XML. Nullable: no todos los correos traen
-- PDF, y los recibos ya persistidos antes de esto tampoco lo tienen.
ALTER TABLE receipts
  ADD COLUMN IF NOT EXISTS raw_pdf BYTEA;
