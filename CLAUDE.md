# CLAUDE.md: CampusBitesBackend

@docs/PROJECT_OVERVIEW.md

Go API for Campus Bites (HTTP layer: Gin). Modular monolith on MySQL. Read the overview above first for the product, the decisions and
the owner's working agreement (design first, honest status, commit/push only when asked).

## Commands
```
go run ./cmd/migrate up        # apply migrations (reads .env). down = roll back ONE step, version = show
go run ./cmd/server            # serve on ADDR (.env sets :8081)
go test ./...                  # unit tests + the architecture test; no database needed
go vet ./... && gofmt -l .     # both must be clean
```
Seed sample data (dev only): `Get-Content db\seed_dev.sql | & "C:\Program Files\MySQL\MySQL Server 8.0\bin\mysql.exe" -u root -p campus_bites_dev`
Config: copy `.env.example` to `.env` (gitignored). Required: `DB_DSN`, `JWT_SECRET` (32+ chars), `RAZORPAY_KEY_ID`,
`RAZORPAY_KEY_SECRET`. Optional: `ADMIN_USERNAME/ADMIN_PASSWORD` (seeds the first admin when the table is empty),
`RAZORPAY_WEBHOOK_SECRET`, `FRONTEND_ORIGIN`, `DEFAULT_CANTEEN`, `ADDR`. Never print or commit secret values.

## Layout
```
cmd/server        wires modules and serves HTTP (graceful shutdown). adapters.go bridges modules.
cmd/migrate       golang-migrate runner; migrations are embedded (migrations/embed.go)
internal/config   env loading
internal/platform apperr (error kinds), httpx (Gin helpers: JSON, errors, Decode, PathID, CORS), authctx, database
internal/canteen  slug -> canteen, per-restaurant branding (name, tagline, logo URL, accent colour) and the admin routes that edit it
internal/auth     admin login, JWT (1h), Require (a gin middleware)
internal/menu     categories/items/variants/add-ons, validation, pricing (PriceLine)
internal/order    orders, lifecycle state machine, pickup codes, tracking, admin views
internal/payment  Razorpay gateway, signature verify, confirm, webhook
migrations/       NNNNNN_name.up.sql / .down.sql
legacy/           first flat version (auth/orders/payment, old schema). Reference only; delete once nothing is left to port.
```
Each module: `model.go` (types), `repository.go` (interface), `mysql.go` (SQL), `service.go` (rules), `handler.go` (HTTP), tests.

## Rules that must not be broken
1. **A module imports only `internal/platform/*`**, never another module (`internal/architecture_test.go` fails otherwise).
   Cross-module needs become a small interface in the *consuming* module, implemented by the other module, adapted in `cmd/server`.
   Existing: `order.Pricer` (menu), `order.Payments` (payment), `payment.OrderNotifier` (order), `menu.CanteenLookup` (canteen).
2. **No foreign keys across module boundaries** and no reading another module's tables. (Within a module, FKs are fine.)
3. **Money is int64 paise.** No floats in DB, JSON or logic.
4. **Prices come from the DB, never from the request.** Requests carry ids and quantities only. Unknown JSON fields are rejected.
5. **Status changes are compare-and-set** (`UPDATE ... WHERE id=? AND status=?`); a lost race is a Conflict, not a silent overwrite.
   Lifecycle lives in `order/model.go` (`transitions`, `adminTargets`); change it there and update `TestCanMove`.
6. **Soft-delete menu rows** (`is_active = 0`); never hard-delete rows that orders may reference.
7. Handlers return errors (`httpx.Handle`); services return `apperr.Invalid/NotFound/Unauthorized/Conflict/Unavailable`. Anything
   else becomes a logged 500 with a generic body. Do not leak internals in messages.
8. Every admin query is scoped by the admin's canteen id taken from the JWT (`authctx`), never from the request.

## Routing (Gin)
- `cmd/server/main.go` builds `gin.New()` with `Recovery` and `httpx.CORS`, then each module's `Register(r *gin.Engine, ...)` mounts its own routes.
  Admin routes use `r.Group("/api/v1/admin", requireAdmin)`; `requireAdmin` is `authSvc.Require` injected from `main.go` so modules never import `auth`.
- Handlers have the shape `func(c *gin.Context) error` and are wrapped with `httpx.Handle`, which turns a returned `apperr` into the JSON error.
  Admin handlers use the module's `withCanteen` wrapper to receive the canteen id from `authctx`.
- Path params are `:id` / `:slug` (`c.Param`), read ids with `httpx.PathID`. Request bodies go through `httpx.Decode` (1MB cap, unknown fields rejected),
  not `ShouldBindJSON`. `*gin.Context` stays in `handler.go`, `httpx` and `auth.Require`; services and repositories never see it.
- The payment webhook reads the raw body (`io.ReadAll`) because the signature covers the exact bytes.
- Gin prints every route at startup in debug mode; set `GIN_MODE=release` in production.
- `legacy/` still uses `net/http` ServeMux on purpose; do not convert it.

## Branding (per restaurant)
`canteens` has optional `tagline`, `logo_url`, `primary_color` (`#rrggbb`). `GET /api/v1/canteens/{slug}` returns them (empty string when unset) and the UI
themes itself from that. Admins edit their own via `GET/PUT /api/v1/admin/canteen` (canteen id from the JWT; validation in `canteen.BrandingInput.validate`,
tested in `canteen_test.go`). The logo is a link only (no uploads yet).

## Adding things
- **Migration:** next number, both `.up.sql` and `.down.sql`, test `up`, `down`, `up` on a scratch database. Never edit an applied migration.
- **Module:** new package under `internal/`, add it to the list in `architecture_test.go`, define the interfaces it needs, wire it in `cmd/server/main.go`.
- **Tests:** unit-test services with hand-written fakes (see `order/service_test.go`, `payment/payment_test.go`). The MySQL layers have no
  automated tests yet; verify them by running the server and calling the API with `curl.exe`.

## Gotchas
- `database.Open` sets `clientFoundRows=true` (so `RowsAffected` counts matched rows: an UPDATE that rewrites identical values is not
  "not found") and `time_zone='+00:00'` (timestamps are UTC; the JSON has `Z`). `multiStatements` is on only for `cmd/migrate`.
- Check `netstat -ano | findstr :8081` before starting: a stale `go run` from an earlier session keeps the port and serves old code (404s on new routes).
- Razorpay payment `verify` uses the provider order id stored in our `payments` table, not the one in the request. Keep it that way.
- The webhook route returns 404 "webhooks are not enabled" until `RAZORPAY_WEBHOOK_SECRET` is set; that is intended.
- `legacy/` has its own tests and env needs; `go test ./...` runs them too. It targets the old `campus_bites` database, not `campus_bites_dev`.
- `GET /api/v1/canteens/{slug}` and the admin canteen routes need migration `000006` applied (`go run ./cmd/migrate up`) or they fail with an unknown-column error.
- Editor/gopls may show "module not in workspace" errors for this folder; `go build ./...` is the truth.

## Next up
On hold, decide together (see "Known gaps" in the overview): resume payment ("Pay now"), idempotency key, pending-order sweeper/expiry,
late-payment handling, and the parked order-tracking security design (per-IP rate limit on `/orders/track`, optionally a per-order secret token).
Propose and agree the remaining decisions (hold time, reinstate vs refund, in-process vs `cmd/worker`, rate-limit numbers, proxy/client IP) before building.

## Git
Branch `feature/gin-router` holds the current work: the Gin conversion plus the branding fields and admin branding routes. It is based on
`feature/modular-menu-orders-payments`, whose PR to `main` is still pending. Commit/push only when asked, on a feature branch.
`.env`, `*.exe` and `/bin/` are gitignored.
