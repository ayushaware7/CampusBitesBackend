# Campus Bites Backend

Go API for Campus Bites: students and staff scan a canteen's QR code, order and pay from their phone.
A modular monolith: one deployable, with modules that own their tables and talk only through Go interfaces, so any of them can be split into its own service later.

UI: [CampusBitesUI](https://github.com/ayushaware7/CampusBitesUI)

## Layout
```
cmd/server/        entry point: loads config, wires modules, serves HTTP
cmd/migrate/       applies migrations (up | down | version)
internal/
  config/          env + .env loading
  platform/        shared plumbing: apperr, httpx, authctx, database
  canteen/         canteens by QR slug
  auth/            admin login, JWT, route protection
  menu/            categories, items, variants, add-ons, pricing rules
  order/           orders, lifecycle (state machine), pickup codes, tracking
  payment/         Razorpay payments, signature verification, webhook
migrations/        numbered SQL files (golang-migrate), embedded in the binary
db/seed_dev.sql    sample data for local development only
legacy/            the first flat version (auth, orders, payment); being ported module by module
```
Rules, enforced by `internal/architecture_test.go`: a module imports only `internal/platform/*`, never another
module, and never reads another module's tables. Modules meet in `cmd/server` through small interfaces they
define themselves (`order.Pricer`, `order.Payments`, `payment.OrderNotifier`, `menu.CanteenLookup`), with adapters
in `cmd/server/adapters.go`. Splitting a module into its own service later means replacing its adapter with a
network client; no module code changes. New tables carry no foreign keys across module boundaries for the same reason.

## Run
```
cp .env.example .env            # then fill in the values
mysql -u root -p -e "CREATE DATABASE campus_bites_dev CHARACTER SET utf8mb4"
go run ./cmd/migrate up
mysql -u root -p campus_bites_dev < db/seed_dev.sql      # optional sample data
go run ./cmd/server
go test ./...
```
`legacy/` still targets the old schema in its own database; do not run it against `campus_bites_dev`.

## Menu API
Money is integer paise. A customer picks a canteen by its QR slug; admins get theirs from the login token.

| Method and path | Auth | Purpose |
|---|---|---|
| `GET /api/v1/canteens/{slug}` | none | Canteen info (QR landing) |
| `GET /api/v1/canteens/{slug}/menu` | none | Full menu: categories, items, variants, add-on groups |
| `POST /auth/login` | none | Returns `{token}` (valid 1 hour) |
| `GET /api/v1/admin/menu` | admin | Own canteen's menu |
| `POST/PUT/DELETE /api/v1/admin/categories[/{id}]` | admin | Delete is refused while it has items |
| `POST/PUT/DELETE /api/v1/admin/items[/{id}]` | admin | PUT syncs variants and add-on group links; ids are stable |
| `PATCH /api/v1/admin/items/{id}/availability` | admin | Sold-out toggle `{"isAvailable": false}` |
| `GET/POST/PUT/DELETE /api/v1/admin/addon-groups[/{id}]` | admin | PUT syncs the group's add-ons |

## Order and payment API
| Method and path | Auth | Purpose |
|---|---|---|
| `POST /api/v1/canteens/{slug}/orders` | none | Place an order (ids and quantities only). The server prices it, stores it as `PENDING_PAYMENT`, opens a Razorpay order and returns `{orderId, code, totalPaise, gatewayOrderId}` |
| `POST /api/v1/payments/verify` | none | After the Razorpay popup: `{orderId, razorpayPaymentId, razorpaySignature}`. Verifies the signature against our stored Razorpay order id, then the order becomes `PLACED` |
| `POST /api/v1/payments/webhook` | signature | Razorpay `payment.captured` safety net; disabled unless `RAZORPAY_WEBHOOK_SECRET` is set |
| `GET /api/v1/canteens/{slug}/orders/track?code=&phone=` | code + phone | Customer order status |
| `GET /api/v1/admin/orders?view=live\|past` | admin | Live = PLACED/PREPARING/READY (oldest first); past = COMPLETED/REJECTED |
| `PATCH /api/v1/admin/orders/{id}/status` | admin | `PREPARING`, `READY`, `COMPLETED` or `REJECTED`; only legal transitions are accepted |

Lifecycle: `PENDING_PAYMENT → PLACED → PREPARING → READY → COMPLETED`; `PLACED` and `PREPARING` can be `REJECTED`
(refund the customer from the Razorpay dashboard for now); an order whose payment setup fails becomes `CANCELLED`.
The order exists before the customer pays, so a payment can never be left without an order, and both the
verify call and the webhook are safe to repeat.

Pricing: unit price = variant price (or the item's base price if it has no variants) + chosen add-ons.
`menu.Service.PriceLine` validates a cart line (variant, add-on min/max, availability) and is what orders will call.

## Configuration (`.env`)
`DB_DSN`, `JWT_SECRET` (32+ chars), `RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET`,
`ADMIN_USERNAME` / `ADMIN_PASSWORD` (first admin, created only if none exists),
optional `RAZORPAY_WEBHOOK_SECRET`, `FRONTEND_ORIGIN`, `DEFAULT_CANTEEN`, `ADDR`.

## Not done yet
- Expiring stale `PENDING_PAYMENT` orders (customers who close the popup), automatic Razorpay refunds on reject,
  rate limiting on the public endpoints, integration tests against a real MySQL (the SQL layers are only tested by hand).
- Notifications (order ready), then delete `legacy/`.
