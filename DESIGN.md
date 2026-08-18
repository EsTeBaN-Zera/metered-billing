# Design

This is a billing core for an API product. Customers burn units. We store each call, roll them into hour windows, and once a month we issue an invoice.

The brief target is 5,000 customers, 200 events/sec (2,000 peak), about 500 million events a month. Accuracy is contractual. I did not build Kafka or a pile of services for that. Postgres can hold this load if writes stay simple and invoices stay correct. The writeup below is where scale lives.

## Stack

Go + Postgres + a small React UI, one repo, `docker compose up`.

The company uses Django. I used Go because I am faster in it on concurrent ingest, unique constraints, and tests that hit a real database. Postgres is the part they actually require. The UI is React + Vite + TypeScript, same as theirs.

Two processes, same database:

- `api` — HTTP (`/v1`, `/ops`, webhook)
- `worker` — dirty hours, then last month's invoices

They do not call each other. A seed command fakes product traffic.

I ingest events **in the request**, not through a queue. A 200 means the row is in Postgres. A queue would only delay the uniqueness check. At 200/sec a batched insert is enough.

## Data model

Main tables:

- `customers` — name + price plan
- `api_keys` — hash + prefix, never plaintext
- `usage_events` — one row per call, `request_id` is the primary key
- `dirty_hours` — hours the worker must rebuild (`customer`, `api_key`, `hour`)
- `usage_windows` — rolled-up units, same grain as dirty hours
- `invoices` + `invoice_line_items`
- `credit_grants` — leftover credit on the customer, used FIFO on the next invoice
- `webhook_events` — `provider_event_id` primary key
- `audit_entries` — insert only

Foreign keys follow the real objects: keys belong to a customer, events belong to a customer and a key, line items belong to an invoice. Money columns are `bigint` with `CHECK (>= 0)` where a negative amount would be wrong. Credits on a line item are allowed to be negative; invoice totals are not.

The brief says one window row per `customer x hour`. I store `customer x hour x api_key`. Billing still sums the month for the customer. `GET /v1/usage?api_key=` can filter without a second table. At 5k customers, 2 keys, 24 x 30 hours that is about 7 million window rows a month, not 3.5 million. That is cheap. A later `customer x hour` rollup is optional if the chart query gets slow. I did not add it.

Events are append-only. Windows are rebuilt from events. An issued invoice is a snapshot. Paid invoices do not change.

`job_locks` is in the schema and unused. The hour job uses `FOR UPDATE SKIP LOCKED` on dirty rows. The month job uses `UNIQUE (customer_id, period_start)`. If two workers both try to issue July for Acme, one insert wins and the other is a no-op. I would wire `job_locks` if I ran many invoice workers and wanted a friendlier "already running" log. The unique key is the real lock.

### Indexes (the queries we run)

- events `(customer_id, api_key_id, event_ts)` — hour recompute is `SUM` over one key and one hour
- windows `(customer_id, hour_bucket)` — monthly sum and the usage chart
- windows `(customer_id, api_key_id, hour_bucket)` — usage filtered by key + cursor
- invoices `(customer_id, issued_at desc)` — list
- `UNIQUE (customer_id, period_start)` — one invoice per month
- `api_keys.key_hash` unique — lookup on every ingest
- `credit_grants.idempotency_key` unique

At 10x I would partition `usage_events` and `usage_windows` by month. At ~50-100x events I would move raw events off the billing primary (see scale).

### Money

Tiers are `$0.001` and `$0.0005`. Cents cannot store that. Amounts are integer microdollars: `$1 = 1_000_000`. `$0.001` per unit is `1000`. Totals are `qty * unit` in integer math. The UI divides by 1e6 to show dollars.

## Pipeline

```
POST /v1/events  ->  usage_events + dirty_hours
worker (often)   ->  SUM events for that hour -> upsert usage_windows
worker (month)   ->  SUM windows for last month -> invoice + lines + apply credits
webhook          ->  invoice status = paid
```

The hour comes from `event_ts` (when the product call happened), not from when we received the HTTP batch. Late and out-of-order events still land in the right hour.

The worker does **not** do `window.units += event`. It recomputes:

```
SUM(units) FROM usage_events
WHERE customer, key, event_ts in [hour, hour+1)
```

then upserts the window. If the job runs twice, the number is the same. If we had incremented, a retry would double-bill.

`dirty_hours.gen` goes up on every new event for that hour. The worker deletes the dirty row only if `gen` still matches. A late event during the job bumps `gen`, the delete misses, and the hour stays dirty. Next run picks it up.

If windows ever disagree with events, the fix is to mark those hours dirty and recompute. I do not "repair" an issued invoice from that (see late events).

Pricing uses the **month total**, not each hour. Plan: 0-10k free, next 90k at `$0.001`, rest at `$0.0005`. Line items are one row per tier that has quantity, then credit lines.

## Idempotency

Rule: do not SELECT then INSERT. Two requests can both see "missing". The unique constraint is the lock. "Already there" is success.

**Ingest replay.** `request_id` is the primary key. `ON CONFLICT DO NOTHING`. Same id twice, even at the same time, is one event. The response counts `inserted` vs `duplicates`. The customer id on the row is the authenticated key, not a body field, so a stolen id from another tenant cannot write into their ledger.

**Hour job twice.** Recompute + upsert. Same totals.

**Invoice job twice.** `UNIQUE (customer_id, period_start)` + `ON CONFLICT DO NOTHING`. One July invoice for Acme.

**Webhook three times.** Insert `provider_event_id`. Conflict means we already saw it. Only the first successful insert may set `issued -> paid`. A second delivery returns 200 and does nothing.

**Ops double-click credit.** `Idempotency-Key` is required. Unique on `credit_grants`. Same key returns the same grant. The UI sends a new UUID per confirm, so two different confirms are two credits on purpose. Two submits of the same confirm are one.

**Override.** `SELECT ... FOR UPDATE` on the invoice. Paid invoices are rejected. Audit stores before/after JSON and a reason.

## Late events

Two doors.

**Door A — month not issued yet (coded).** Event is stored. Dirty hour is marked. Window is rebuilt. The invoice job has not run, so the month total will include it.

**Door B — invoice already issued (described, not coded).** We still store the event and refresh the window (the meter should be true). We do **not** rewrite the invoice the customer already saw. Extra units become an adjustment on a later period, or ops issues a credit / debit by policy. Accuracy is contractual; a PDF that changes after send is a fight.

I did not build Door B. The brief said we do not need a full reconciliation system. The important part is: windows move, issued invoices do not.

## Failure modes at the brief's numbers

500 million events/month is ~19 events/sec average, 200 sustained, 2,000 peak. Windows are ~7 million rows/month. Invoices are 5,000 rows/month.

**1. `usage_events` on the same disk as invoices (breaks first).** Ingest is a lot of small writes. Invoice jobs, RLS, and backups share that primary. At ~10x (2,000/sec sustained) WAL and autovacuum on events hurt. Fix: partition events by month, then put ingest on a queue + writer pool still in Postgres. At ~50-100x, events leave the billing database; the worker reads from the event store and only writes windows + invoices to billing Postgres. Billing schema does not change.

**2. Hour job lag, not invoice math.** 2,000/sec peak dirties many hours. If the worker falls behind, `/v1/usage` looks low until catch-up. Invoices wait until windows exist, so they stay correct but late. Fix: more workers (SKIP LOCKED already allows that), then partition dirty work by customer hash.

**3. One fat customer.** One key at peak can serialize on that customer's hour rows. Fix: keep A1 grain (already per key), and cap batch size (already 500). I would add a per-key rate limit in the product, not in billing, if one customer can drown ingest.

What will **not** break first: invoice row count, credit grants, audit. Those stay small.

This design scales with known fixes. It does not "not scale." The first fix is operational (partition, more workers). The rewrite (events off the billing primary) is later, and it is not microservices of invoices.

## Threat model

### Hostile customer

Worst: read Globex invoices by guessing UUIDs, or replay a batch to inflate Globex / deflate themselves.

Stops them:

- `/v1` never trusts `customer_id` in the URL or body. Bearer key -> hash -> `customer_id`.
- That id is `SET LOCAL app.customer_id` in the same transaction.
- RLS on events, windows, invoices, keys, credits: `customer_id = current_customer_id()` unless ops. A handler that forgets `WHERE customer_id = ...` still returns no row. Guessing an id is **404**, not 403, so we do not leak that the invoice exists.
- Keys are SHA-256(plaintext + `KEY_PEPPER`). Pepper is env, not in the repo. Prefix is for display. Plaintext is printed once by seed and never stored.
- CORS allowlist. A browser from a random origin gets 403. curl with no Origin still works (webhooks, scripts).

They can still burn their own key and generate a big bill. That is their account.

### Hostile ops user

Worst: silent credit to a friend, edit a paid invoice, wipe the audit trail, dump all keys.

Stops them:

- Ops is a shared `OPS_TOKEN`, not per-user SSO. That is a gap (see what I did not build). The token is env-only and is not a customer key.
- Credit needs a reason and an idempotency key. Override needs a reason. Paid invoices cannot be patched.
- `audit_entries`: trigger rejects UPDATE/DELETE. App role has INSERT + SELECT only, no UPDATE/DELETE.
- `lookup_api_key` is `SECURITY DEFINER` so ingest can hash-lookup without reading other tenants' key rows through RLS. The app never SELECTs plaintext because it does not exist.

They can still issue a bad credit if they have the token. The audit row stays. That is the control we have without real identity.

### Compromised webhook sender

Worst: mark invoices paid without money, or replay an old paid event onto another invoice.

Stops them:

- `X-Webhook-Signature` HMAC-SHA256 of the raw body, secret from env. Bad sig is 401.
- Replay of the same `event_id` hits `webhook_events` unique and does not pay twice.
- The body names one `invoice_id`. We do not take "pay all." A stolen old payload cannot switch ids without a new valid signature.
- We do not accept unsigned JSON, and we do not log the secret.

If the secret leaks, they can pay any invoice. Rotate the secret. There is no per-customer webhook key in this version.

## APIs and UI

Customer: `POST /v1/events`, `GET /v1/usage`, `GET /v1/invoices`, `GET /v1/invoices/{id}`.

Usage is cursor pagination `(hour, api_key_id)`. Hour series can be large; offset would skip/duplicate under inserts. Invoices are offset. There are a few dozen a year per customer. I did not use a cursor there.

Ops: list/detail customers, credits, invoice get, line override, signed payments webhook.

Money-moving UI (credit, override) is a confirm modal plus a reason. Credit sends `Idempotency-Key`. Buttons disable while the request is in flight. Errors show in the page. Ops token and API keys are not in the repo; they live in env / the seed stdout.

## Trade-offs

**1. Recompute dirty hours, not `+=`.**  
Rejected incrementing the window on ingest. Faster on the write path, wrong on retry. The job already has to run. Recompute is the idempotent version of that job.

**2. Freeze issued invoices.**  
Rejected "reopen March when a late event arrives." The window updates. The invoice the customer saw does not. Correction is next period or a credit. That matches "accuracy is contractual" better than a mutating PDF.

**3. Sync ingest to Postgres, queue later.**  
Rejected HTTP 202 + Kafka on day one. Uniqueness still needs the database. At 200/sec the extra hop hides bugs and does not remove the unique key. The evolutionary path is in the failure-modes section.

**4. Go, not Django.**  
Rejected copying the team's framework. I am slower in Django for this kind of concurrent write test. Cost: no `Manager` for tenant scope, so I put RLS in Postgres instead. That is the right layer anyway (cannot be forgotten in a view).

## What I did not build (and would next)

- Door B adjustments after issue
- Real ops users / RBAC (today: one token)
- Prepaid / kill-switch (this product is postpaid monthly)
- Key rotation UI (seed prints plaintext once)
- Using `job_locks`
- A metrics product. Alerts below are the hooks I would add, not a Grafana stack in this repo
- Moving events off Postgres (only when numbers say so)

Next if this shipped: Door B as a next-period line item, ops login, partition `usage_events` by month.

## Alerts and debugging a wrong invoice

I would alert on:

- ingest 5xx or latency p99 (events not landing)
- `dirty_hours` age > 15 minutes (usage UI lying)
- invoice job failed / issued count = 0 on the first of the month
- webhook 401 spike (bad secret or attacker)
- `audit_entries` insert errors
- one customer ingest share > 30% of QPS (noisy neighbor)

To debug a wrong invoice: open the invoice id in ops, read line items, `SUM(usage_windows)` for that `customer_id` and `[period_start, period_end)`, compare to the tier math. If the window is wrong, `SUM(usage_events)` for those hours. If events look right and windows do not, mark the hours dirty and recompute. If the invoice disagrees with windows and it is already issued, do not UPDATE the invoice; credit or next-period adjust, and look at audit for an override.
