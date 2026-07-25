# Rhix — backend

Ingesta automática de Recibos por Honorarios (XML/UBL de SUNAT) recibidos por
correo. Un worker lee un buzón IMAP dedicado, extrae los adjuntos `.xml` de
los correos sin leer, los persiste en Postgres, y una API delgada expone lo
ya procesado al frontend.

## Arquitectura

Dos binarios independientes, mismo módulo Go:

- **`cmd/worker`** — Cron Job de corta duración (pensado para Railway Cron).
  Cada corrida: toma un `pg_try_advisory_lock` para evitar solaparse con otra
  corrida en curso, se conecta por IMAP, busca los correos `UNSEEN`, extrae
  los adjuntos `.xml` y los datos del correo (from/to/cc/asunto/cuerpo), los
  parsea y persiste, y recién ahí marca el correo como `SEEN`.
- **`cmd/api`** — servicio web persistente. Solo lee lo que el worker ya
  persistió; no tiene ni necesita credenciales IMAP.

```
cmd/
  api/main.go      # GET /health, /api/receipts, /api/receipts/export,
                    # /api/receipts/{id}/xml, /api/receipts/{id}/email
  worker/main.go   # ciclo de ingesta
internal/
  config/          # carga y valida env vars (separado por binario)
  lock/            # advisory lock de Postgres, anti-solapamiento
  inbox/           # sesión IMAP: login, SEARCH UNSEEN, FETCH, extrae adjuntos .xml
  parser/          # interpreta el XML UBL de un Recibo por Honorarios Electrónico
  receipt/         # modelo Receipt + repositorio (Exists/Create/List)
  excel/           # genera el .xlsx exportable
  db/              # conexión a Postgres (protocolo simple, ver nota abajo)
migrations/        # SQL versionado a mano, se corre manualmente contra Supabase
```

## Estado actual

- [x] **Fase 1** — scaffold, conexión a Postgres (Supabase), lock, repositorio de recibos, migración inicial.
- [x] **Fase 2** — conexión IMAP real (`internal/inbox`): login, `SEARCH UNSEEN`, `FETCH` + parseo MIME, extracción de adjuntos `.xml` en memoria.
- [x] **Fase 3** — parser UBL de Recibo por Honorarios Electrónico (`internal/parser`), persistencia vía `internal/receipt` con manejo de duplicados, y `MarkSeen` solo tras persistir con éxito. Verificado extremo a extremo contra Gmail y Supabase reales.
- [x] **Fase 4** — `GET /api/receipts/export` (`internal/excel`, con `xuri/excelize`) descarga un `.xlsx` con todos los recibos.
- [x] **Correo original** — `GET /api/receipts/{id}/xml` (el XML crudo), `GET /api/receipts/{id}/pdf` (el PDF, si el correo traía uno) y `GET /api/receipts/{id}/email` (from/to/cc/asunto/cuerpo + lista de adjuntos del mismo correo, con `hasPdf`) para verlo todo desde el frontend. Solo aplica a recibos procesados desde que se agregó esto — los anteriores no tienen estos datos guardados.

### Nota sobre el Transaction Pooler

`internal/db` fuerza `pgx.QueryExecModeSimpleProtocol`. Es necesario: el modo
por defecto de pgx usa sentencias preparadas nombradas, que el Transaction
Pooler de Supabase (PgBouncer en modo transacción) no soporta entre
conexiones — sin esto, la segunda corrida del worker falla con `prepared
statement already exists`.

## Requisitos

- Go (ver `go.mod` para la versión exacta)
- Un proyecto de Supabase (Postgres) — usar el connection string del
  **Transaction Pooler** (puerto `6543`), no la conexión directa.
- Un buzón de Gmail dedicado con verificación en 2 pasos activada y una
  **contraseña de aplicación** generada (no la contraseña normal de la cuenta).

## Configuración local

```bash
cp .env.example .env
```

Completa `.env` con:

| Variable | De dónde sale |
|---|---|
| `DATABASE_URL` | Supabase → Project Settings → Database → Connection string → **Transaction Pooler** |
| `IMAP_HOST` / `IMAP_PORT` | `imap.gmail.com` / `993` |
| `IMAP_USER` | el correo del buzón dedicado |
| `IMAP_PASSWORD` | contraseña de aplicación de Gmail (16 caracteres, sin espacios) |
| `PORT` | solo local; en Railway la inyecta la plataforma |

`cmd/api` solo requiere `DATABASE_URL`. `cmd/worker` requiere además las
variables `IMAP_*`.

## Migraciones

No hay herramienta de migraciones automatizada todavía — los archivos en
`migrations/` se corren a mano contra la base:

```bash
set -a && source .env && set +a
psql "$DATABASE_URL" -f migrations/0001_create_receipts.sql
psql "$DATABASE_URL" -f migrations/0002_add_email_details.sql
psql "$DATABASE_URL" -f migrations/0003_add_raw_pdf.sql
```

## Correr en local

```bash
go run ./cmd/worker   # un ciclo de ingesta y termina
go run ./cmd/api      # queda escuchando en :8080 ($PORT)
```

## Deploy

Pensado para Railway:
- `cmd/worker` como **Cron Job** (proceso corto, no daemon).
- `cmd/api` como **servicio web** persistente.
