-- Los recibos nuevos guardan el XML/PDF en Supabase Storage (bucket privado
-- "recibos"), no en la fila directo. raw_xml/raw_pdf quedan para los recibos
-- ya persistidos antes de este cambio; el código prefiere el storage_path
-- cuando existe y usa la columna vieja como respaldo si no.
ALTER TABLE receipts
  ADD COLUMN IF NOT EXISTS xml_storage_path TEXT,
  ADD COLUMN IF NOT EXISTS pdf_storage_path TEXT;

-- raw_xml era NOT NULL porque antes era la única forma de guardar el XML;
-- los recibos nuevos van a tener xml_storage_path en su lugar y dejan
-- raw_xml en NULL.
ALTER TABLE receipts
  ALTER COLUMN raw_xml DROP NOT NULL;
