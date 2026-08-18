# metered-billing

```
docker compose up --build
docker compose --profile seed run --rm seed
```

Copy `.env.example` to `.env` if you want to change secrets.

- API: http://localhost:8080
- UI: http://localhost:8088  (`#/ops` for ops)
- Seed prints API keys once. Default ops token is `OPS_TOKEN` in `.env.example`.

Money is integer microdollars (`$1` = `1000000`).

If you change SQL after the first Postgres start:

```
docker compose down -v
docker compose up --build
```
