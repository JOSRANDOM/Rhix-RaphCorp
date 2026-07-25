-- Guarda los datos del correo original (no solo del recibo) para poder
-- mostrarlo completo en el frontend: de quién viene, para quién, copia,
-- asunto y cuerpo. Los recibos ya existentes quedan con estos campos vacíos
-- — no hay forma de recuperar esos datos retroactivamente.
ALTER TABLE receipts
  ADD COLUMN IF NOT EXISTS email_from TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_to TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_cc TEXT,
  ADD COLUMN IF NOT EXISTS email_subject TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS email_body TEXT NOT NULL DEFAULT '';
