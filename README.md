# Campus Bites Backend

Go API for Campus Bites: multi-canteen menu, Razorpay payments, live and past orders, and canteen admin login.

UI: [CampusBitesUI](https://github.com/ayushaware7/CampusBitesUI)

## Run
1. Copy `.env.example` to `.env` and fill in the values (never commit `.env`).
2. Create the schema (MySQL): `mysql -u root -p < schema.sql`
3. `go run .` (listens on `:8080`)

Tests: `go test ./...`

## Configuration (`.env`)
| Variable | Purpose |
|---|---|
| `DB_DSN` | MySQL connection, `user:password@tcp(host:port)/campus_bites` |
| `JWT_SECRET` | At least 32 characters; signs admin tokens |
| `RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET` | Razorpay API keys |
| `ADMIN_USERNAME`, `ADMIN_PASSWORD` | First admin, created only if the `admins` table is empty |
| `FRONTEND_ORIGIN`, `DEFAULT_CANTEEN`, `ADDR` | Optional |

## API
Customers pick a canteen with the `X-Canteen` header (set by the UI after a QR scan); admins get theirs from the JWT.

| Method and path | Auth |
|---|---|
| `GET /api/v1/items`, `GET /api/v1/category/{c}` | none |
| `POST /api/v1/items` | admin |
| `POST /api/create-order`, `POST /api/verify-payment` | none (payment verified by signature, amount checked against DB prices) |
| `GET /api/admin/my-orders?phone=` | none |
| `POST /auth/login` | none |
| `GET /api/admin/live-orders`, `GET /api/admin/past-orders` | admin |
| `POST /api/admin/complete/{id}`, `POST /api/admin/reject/{id}` | admin |

## Status
First working version, flat file layout. Next: restructure as a modular monolith (auth, menu, order, payment) with migrations and tests.
