# metered-billing

Metered API billing: ingest usage, roll it into hour windows, issue a monthly invoice.

How it is designed (pipeline, schema, scale path): **[DESIGN.md](DESIGN.md)**.

## Run after clone

You need **Docker Desktop** (Compose v2) and Git. Nothing else: the API, worker, Postgres, and UI all start from Compose.

```bash
git clone https://github.com/EsTeBaN-Zera/metered-billing.git
cd metered-billing
copy .env.example .env
docker compose up --build
```

On macOS/Linux use `cp .env.example .env` instead of `copy`.

Wait until `api` is listening. In another terminal, load demo customers and print API keys **once**:

```bash
docker compose --profile seed run --rm seed
```

If seed says `seed already ran, skip`, the keys will not print again. Reset the database (this deletes data) and seed again:

```bash
docker compose down -v
docker compose up --build
docker compose --profile seed run --rm seed
```

### URLs

| What | Where |
| --- | --- |
| Customer UI | http://localhost:8088 |
| Ops UI | http://localhost:8088/#/ops |
| API | http://localhost:8080 |
| Health | http://localhost:8080/health |
| Postgres | `localhost:5432` user `app` / password `app` database `billing` |

Ops token (default in `.env.example`): `dev-ops-token-change-me`. Paste it in the ops page.

Customer UI: paste one `sk_live_…` key from the seed stdout. Harborline has two keys; the others have one.

Money in the API is integer microdollars (`$1` = `1_000_000`). The UI shows dollars.

### Tests

With Compose `db` up (or `docker compose up -d db`):

```bash
go test ./...
```

Tests use `postgres://app:app@localhost:5432/billing`. They create extra `test-*` customers on that same database; they are not the seed firms. In ops, leave “Show test customers” off if you only want Harborline / Cinder / Quill / Kestrel.

### If you change SQL

Migrations run only on an empty volume. After editing `migrations/`:

```bash
docker compose down -v
docker compose up --build
docker compose --profile seed run --rm seed
```
