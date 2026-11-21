# Walking skeleton — remaining steps

Goal: a deployable Go server that talks to Postgres, serves one HTML page and a
health check, runs on Cloud Run, and can be opened on a phone. No auth, shelf,
planner or polling yet.

Definition of done:

- `make check` passes.
- `make migrate` applies `0001_initial.sql`.
- `make dev` serves `/` (HTML), `/healthz` (JSON), `/static/...`, and a 404 page.
- `docker build` succeeds and the container runs locally.
- The Cloud Run URL loads on a phone and `/healthz` returns 200.
- No secrets are committed.

---

## Current state

Already in the repo:

- `main.go` — flags, config, logging, HTTP server, graceful shutdown.
- `internal/config` — `.env.local` loading and validation.
- `internal/logging` — `slog` to stdout.
- `internal/httpserver` — middleware chain, `/health`, error template.
- `internal/httpx` — JSON and error-page helpers.
- `internal/templates` — template renderer.
- `internal/database/db` — sqlc-generated pgx code.
- `internal/database/queries/users.sql`, `internal/database/migrations/0001_initial.sql`.
- `make generate`, `make generate-check`, `make migrate*`.

Blocking the build right now:

```text
main.go:39:29: undefined: database
main.go:44:12: undefined: database
main.go:57:12: app.Close undefined (*httpserver.Application has no Close)
```

Not done yet: database pool, migration applied, home page, static serving,
`/healthz`, tests, Dockerfile, README, deploy.

---

## 1. Fix the build (database + config + lifecycle)

- [ ] Create `internal/database/database.go`:

```go
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Open(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}

	cfg.MaxConns = 5
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = time.Minute

	// Supavisor transaction mode (port 6543) cannot keep prepared statements
	// alive across connections. Exec mode sends Parse/Bind/Execute each time.
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}
```

### Why `pgxpool`

`pgxpool` is the app-side pool, not a duplicate of Supabase's pooler. Supavisor
(port 6543, transaction mode) multiplexes connections at the database edge;
`pgxpool` still reuses a small number of connections inside each Cloud Run
instance and bounds concurrency. `PLAN.md` §10.1 sizes it at ≤5 per instance
with 2–3 Cloud Run instances.

It is also the type `sqlc` expects: `sqlc.yaml` uses `sql_package: "pgx/v5"`,
so generated queries use `pgx.Row`, `pgx.Rows`, and `pgx.Tx`. A `database/sql`
pool would require changing `sqlc.yaml` to `sql_package: "database/sql"` and
would lose pgx's native `int[]`/`text[]` handling.

The `database/sql` import and `*sql.DB` parameter currently in
`internal/httpserver/app.go` are stale placeholders. Step 1 replaces them with
the `Pinger` interface below; `*pgxpool.Pool` satisfies it.

- [ ] Do **not** add `Migrate` to this package. Migrations run only through
      `make migrate` against `DIRECT_URL` (§10.1 of `PLAN.md`).
- [ ] Remove the `database.Migrate(...)` call from `main.go`.
- [ ] Replace `appConfig.DatabasePath` with `appConfig.DatabaseURL` in `main.go`.
- [ ] Keep `defer pool.Close()` in `main.go`.

- [ ] Change `internal/config/config.go`:
  - Rename `DatabasePath` to `DatabaseURL`.
  - Delete `defaultDatabaseFilename` and the SQLite fallback.
  - Require `DATABASE_URL` (return an error if empty).
  - Drop the now-unused `workingDirectory` argument from `Parse`, or leave it
    and use `_`.
  - Keep `PORT`, `APP_URL`, `AUTH_SECRET`, and the `.env.local` loader.

- [ ] Change `internal/httpserver/app.go`:
  - Replace `*sql.DB` with a small interface:

```go
type Pinger interface {
	Ping(context.Context) error
}
```

  - `func New(db Pinger, logger *slog.Logger, options Options) (*Application, error)`.
    `*pgxpool.Pool` satisfies this.
  - Replace `log.Fatal("cannot set up renderer")` with a returned error.
  - Add:

```go
func (a *Application) Close() error {
	return a.publicRoot.Close()
}
```

  - The `db` argument must now be used; see `/healthz` below.

- [ ] Build and run:

```bash
go build ./...
make migrate
make dev
```

`make migrate` needs `DIRECT_URL` in `.env.local`. `make dev` needs
`DATABASE_URL` in `.env.local`.

---

## 2. Server surface

- [ ] Add `GET /healthz` and remove/alias `GET /health`.
      The plan uses `/healthz` for the Cloud Scheduler keepalive, so it should
      ping the database:

```go
mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := db.Ping(ctx); err != nil {
		logger.Error("healthz: database ping failed", "error", err)
		httpx.RespondWithJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false})
		return
	}
	httpx.RespondWithJSON(w, http.StatusOK, map[string]any{
		"ok": true, "app": "boardgamesort",
	})
})
```

- [ ] Add `GET /` as a real page. Use `GET /{$}` so it matches only the root,
      not every path:

```go
mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
	if err := renderer.Render(w, http.StatusOK, "home", templates.Page{
		Title: "BoardgameSort",
	}); err != nil {
		logger.Error("render home", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
})
```

- [ ] Create `web/templates/home.html` with `{{define "home"}}...{{end}}`.
      Keep it minimal and do not use inline styles: the CSP is
      `style-src 'self'`.
- [ ] Serve static files from `web/public`:

```go
mux.Handle("GET /static/", http.StripPrefix(
	"/static/",
	http.FileServerFS(publicRoot.FS()),
))
```

- [ ] Add a 404 catch-all **after** the specific routes:

```go
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	if err := httpx.RespondWithErrorPage(
		w, renderer, http.StatusNotFound, "Not Found", "That page does not exist.",
	); err != nil {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
})
```

- [ ] Check the middleware chain still is `CspNonce → SecurityHeaders →
      RecoverPanics`. Do not wire `GlobalSourceValidation` yet; leave it
      defined and unused.
- [ ] Add a tiny `web/public/app.css` (or delete the need for one) so the
      static route is exercised. `web/public/.gitkeep` can stay.

---

## 3. Tests and dev ergonomics

- [ ] Add `internal/httpserver/app_test.go`:
  - Stub `Pinger` with success and failure modes.
  - `/healthz` returns 200 when `Ping` succeeds.
  - `/healthz` returns 503 when `Ping` fails.
  - `/` returns 200 and contains `BoardgameSort`.
  - `/does-not-exist` returns 404.
  - Use `Options{TemplateDirectory: "../../web/templates", PublicDirectory: "../../web/public"}`.
  - Call `app.Close()` in cleanup.
- [ ] Add `internal/config/config_test.go`:
  - Valid `DATABASE_URL`/`APP_URL`/`AUTH_SECRET` parses.
  - Missing `DATABASE_URL` fails.
  - Missing `AUTH_SECRET` fails.
  - Invalid `APP_URL` fails.
- [ ] Add `.env.example`:

```dotenv
PORT=8080
APP_URL=http://localhost:8080
AUTH_SECRET=replace-with-32-plus-random-bytes
DATABASE_URL=postgres://app:app@localhost:5432/boardgames?sslmode=disable
DIRECT_URL=postgres://app:app@localhost:5432/boardgames?sslmode=disable
```

  Generate a real local secret with `openssl rand -base64 32`. Never commit
  `.env.local`.

- [ ] Add `README.md` with:
  - what the app is;
  - prerequisites (Go, Postgres, `goose`, `sqlc`, Docker optional);
  - local database setup;
  - copy `.env.example` to `.env.local`;
  - `make migrate`, `make dev`, `make check`;
  - the layout from `PLAN.md` §10;
  - a pointer to `PLAN.md`.

- [ ] Add `AGENTS.md` restating the review contract:
  - `make check` is the gate;
  - schema changes only in a new numbered migration;
  - `internal/database/db/` is generated — run `make generate`, never edit;
  - migrations only through `DIRECT_URL`, never from the app;
  - every generated link/QR uses `APP_URL`;
  - keep `GlobalSourceValidation` untouched;
  - no secrets in git.

- [ ] Clean up the Makefile: `make seed` currently points at `cmd/seed`, which
      does not exist. Either implement `cmd/seed` in the auth step or remove
      the target until then.

---

## 4. Container

- [ ] Add `Dockerfile`:

```dockerfile
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/boardgamesort .

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/boardgamesort /app/boardgamesort
COPY --from=build /src/web /app/web
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/boardgamesort"]
```

- [ ] Add `.dockerignore`:

```text
.git
.env
.env.*
bin/
data/
*.md
```

  Do **not** exclude `web/`. The app currently loads templates and static files
  from disk at `web/templates` and `web/public`, relative to the working
  directory.

- [ ] Build and run locally:

```bash
docker build -t boardgamesort .
docker run --rm -p 8080:8080 --env-file .env.local boardgamesort
```

  On Linux, if Postgres is on the host, add
  `--add-host=host.docker.internal:host-gateway` and use
  `host.docker.internal` in `DATABASE_URL`. Or point `DATABASE_URL` at Supabase.

- [ ] Optional, after the skeleton is deployed: embed `web/` with `go:embed`
      so the image can be a single binary. Not required for the first deploy.

---

## 5. Cloud Run + Supabase

- [ ] Create/choose a GCP project, enable billing, and enable:
      Cloud Run, Cloud Build, Artifact Registry, Secret Manager,
      Cloud Scheduler.
- [ ] Create an Artifact Registry repository if you are not using
      `gcloud run deploy --source .`.
- [ ] Create the Supabase project. Keep the region the same as Cloud Run.
- [ ] Apply the schema from your laptop/CI with the **direct** URL:

```bash
DIRECT_URL='postgres://...:5432/postgres' make migrate
make migrate-status
```

  Confirm `users`, `groups`, `memberships`, `invites`, `auth_sessions`,
  `games`, `copies`, `vetoes` exist.

- [ ] Create secrets:

```bash
openssl rand -base64 32 | tr -d '\n' | gcloud secrets create AUTH_SECRET --data-file=-
gcloud secrets create DATABASE_URL --data-file=-   # pooled URL, port 6543
```

  Grant the Cloud Run service account
  `roles/secretmanager.secretAccessor`.

- [ ] Deploy:

```bash
gcloud run deploy boardgamesort \
  --source . \
  --region <region> \
  --allow-unauthenticated \
  --min-instances=0 \
  --max-instances=3 \
  --memory=512Mi \
  --cpu=1 \
  --timeout=30s \
  --set-secrets=DATABASE_URL=DATABASE_URL:latest,AUTH_SECRET=AUTH_SECRET:latest \
  --set-env-vars=APP_URL=http://localhost:8080
```

  `PORT` is injected by Cloud Run. Do **not** set `DIRECT_URL` on the service.

- [ ] Read the service URL and update `APP_URL`:

```bash
URL=$(gcloud run services describe boardgamesort \
  --region <region> --format='value(status.url)')
gcloud run services update boardgamesort \
  --region <region> --update-env-vars=APP_URL="$URL"
```

- [ ] Add the daily keepalive:

```bash
gcloud scheduler jobs create http boardgamesort-keepalive \
  --schedule='0 9 * * *' \
  --uri="$URL/healthz" \
  --http-method=GET \
  --location=<region>
```

  This hits Postgres (`SELECT 1` via `Ping`), so it also prevents the free
  Supabase project from pausing. If you later add a Cloud Run liveness probe,
  point it at a DB-free `/livez` instead, so a database blip does not restart
  instances.

- [ ] Set a $1 budget alert.

---

## 6. Verify the skeleton

- [ ] `make check` is green (fmt, vet, sqlc diff, tests, build).
- [ ] `make migrate-status` shows `0001_initial.sql` applied.
- [ ] `make dev` and:

```bash
curl -i http://localhost:8080/
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/does-not-exist
```

- [ ] `docker build` and `docker run` work.
- [ ] Open the Cloud Run URL on a phone over cellular.
- [ ] Cloud Scheduler job succeeds.
- [ ] `git status` shows no `.env.local`, secrets or build artifacts.

---

## Notes and decisions

- **No migrations from the app.** `main.go` must not call `Migrate`. This is the
  plan's rule and the reason `DIRECT_URL` is separate.
- **`/healthz` pings the DB.** That is deliberate for the Supabase keepalive.
  Add `/livez` only if Cloud Run probes need a DB-free endpoint.
- **Prepared statements:** `QueryExecModeExec` is required for Supabase's
  transaction pooler and works locally too.
- **Templates/static are on disk** for the skeleton. The Dockerfile copies
  `web/`. Embedding them with `go:embed` is a later, optional simplification.
- **Tailwind is not wired yet.** The skeleton can ship with a plain CSS file;
  the standalone Tailwind build belongs with the first real screens.
- **`AUTH_SECRET` length** is only checked as non-empty today. Enforce 32+
  bytes before deploying, or keep the check in config and update `.env.local`.
- **`GlobalSourceValidation`** stays defined and unused until the auth/CSRF
  step. Do not wire it just to remove the “unused” look.
