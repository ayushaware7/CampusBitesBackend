# Campus Bites: project overview

> This file is identical in both repos (`CampusBitesBackend/docs` and `CampusBitesUI/docs`). If you change one, change the other.
> Last updated 2026-10-31. Check `git log` and the open pull requests for anything newer.

## What it is
A pre-ordering app for campus canteens and restaurants that removes the queue.
A restaurant or canteen prints a QR code that points at `https://<frontend>/r/<slug>` (the older `/c/<slug>` still works). A customer scans it, the
app opens that restaurant's menu, they pick items (with sizes and add-ons), pay online with Razorpay, and collect the food
when the order is marked ready. No login for customers. Restaurant staff log in to manage the menu and the orders.

## The repos
| Repo | What | Local folder | GitHub |
|---|---|---|---|
| CampusBitesBackend | Go API (Gin, modular monolith, MySQL) | `Desktop\Projects\CampusBitesBackend` | `ayushaware7/CampusBitesBackend` |
| CampusBitesUI | React single-page app (customer app + admin) | `Desktop\Projects\CampusBitesUI` | `ayushaware7/CampusBitesUI` |

A third project, `Desktop\Projects\YourBiteWebsite`, is the **Your Bite company website** (see "Two things we are building"). It is a separate
codebase, scaffolded but not in git yet; its repo URL will be added here once created.

Each Campus Bites repo has its own `CLAUDE.md` with the working details. `Desktop\campusBites` and `Desktop\Minor` are old copies of the
first prototype (Java backend, early React); ignore them.

## Two things we are building (decided 2026-10-31)
- **Your Bite** is the company (generic name, not campus-specific). Its **website** shows who we are, our services, why choose us, the team
  and contact details. It is marketing only and lives in its own project (`YourBiteWebsite`: Vite + React + Tailwind, static, all text in
  `src/content.js`, placeholders marked `[Placeholder]`).
- **Campus Bites** stays as the **product** name and repo names. It is the working app (menu, ordering, payment, tracking, and a dedicated admin)
  that we run ourselves and show to clients. We tell clients their branding replaces Campus Bites; the structure stays the same.
- The product has **no marketing pages**: the old landing and About pages were deleted from the UI (they are in git history if text is needed).
  `/` opens the default canteen's menu.
- **Per-client branding comes from the backend** (not a config file): each canteen row has name, tagline, logo URL and accent colour; the customer
  app themes itself from `GET /canteens/{slug}`; the admin edits them in the **Branding** tab. Customer-facing wording says "restaurant"; the code,
  database and API keep the word "canteen" internally on purpose.

## How a customer's order flows
1. Scan QR, then `/r/:slug` (or `/c/:slug`): the UI stores the canteen slug and opens the menu (`GET /canteens/{slug}/menu`).
2. Customer builds a cart. A cart line is item + chosen size + chosen add-ons + quantity.
3. Checkout: the UI sends only ids and quantities; **the server prices everything** from the live menu, stores the order as
   `PENDING_PAYMENT`, creates a Razorpay order, and returns the gateway order id.
4. Razorpay popup; then the UI calls `POST /payments/verify`; the server checks the signature against the Razorpay order
   id it stored itself, and the order becomes `PLACED`.
5. The canteen sees it under Live Orders and moves it `PREPARING` then `READY` then `COMPLETED` (or `REJECTED`).
6. The customer follows progress on Track Orders using the **pickup code + phone number**.

Order statuses: `PENDING_PAYMENT, PLACED, PREPARING, READY, COMPLETED, REJECTED, CANCELLED`.
"Live" = PLACED/PREPARING/READY. Admins never see unpaid orders.

## Architecture decisions (and why)
- **Gin for HTTP** (switched from the standard library mux on 2026-10-31 because the owner knows Gin). Gin is confined to `handler.go` files,
  `httpx` and the auth middleware; services, repositories and SQL do not depend on it. Handlers return errors and `httpx.Handle` writes them.
- **Modular monolith, not microservices.** One Go binary with modules `auth, canteen, menu, order, payment`. Modules never
  import each other and never read each other's tables; they meet in `cmd/server` through small interfaces they define
  themselves. This keeps each one splittable into its own service later. A test (`internal/architecture_test.go`) enforces it.
- **MySQL for everything** for now (orders, payments and admins are relational and money-related). MongoDB was considered
  and rejected for the menu; revisit only for an event log or notifications.
- **Money is integer paise** everywhere (DB, API, cart). Rupees exist only for display and form fields.
- **The order exists before payment.** The old prototype saved the order only after payment, so a failure in between left a
  customer who paid with no order. Now a payment can never be orphaned.
- **Server-side pricing.** Never trust a price from the browser.
- **Soft deletes** (`is_active = 0`) for menu rows so old orders and carts still resolve. Order lines are a snapshot of names
  and prices at order time.
- **One `orders` table with a status**, not separate live/past tables.
- **Images:** the menu stores an `image_url` only. Large image files are not committed to git; images will move to
  database/object storage later. Two large photos (`Pizza.jpg`, `chole-bhature.jpg`) were deliberately not pushed.
- **No public admin registration.** The first admin is seeded from `.env`.

## Current status (2026-10-31)
Done and working end to end: QR landing, menu with variants/add-ons/sold-out/specials, cart, checkout with Razorpay test
payments, order tracking, admin login, admin menu + add-on management, admin live/past orders with a live-count badge.

Added on 2026-10-31 (owner tested the UI and said it looked fine; the new admin Branding save flow and a real payment after the Gin switch were
checked by curl/build only, so re-test them if something looks off):
- **Backend moved to Gin** (same URLs, methods and JSON as before).
- **Product/company split:** marketing pages removed from the UI; `/` opens the menu; `/r/:slug` added next to `/c/:slug`; admin code is lazy-loaded
  behind its own route section.
- **Per-restaurant branding:** migration `000006`, branding fields on `GET /canteens/{slug}`, admin `GET/PUT /admin/canteen`, UI `BrandContext`
  (name, logo, tagline, accent colour via `brand-*` Tailwind classes driven by CSS variables) and an admin **Branding** tab.
- **Your Bite website** scaffolded in `YourBiteWebsite` with placeholder content (not pushed anywhere yet).

Branches: backend `feature/gin-router` (on top of `feature/modular-menu-orders-payments`), UI `feature/product-rebrand` (on top of
`feature/new-backend-menu-orders`). Pull requests into `main` are still to open/merge; until then `main` holds the old flat design.

## Known gaps and the next piece of work
1. **Unpaid orders are stuck.** If a customer closes the Razorpay popup, the order stays `PENDING_PAYMENT` forever, is invisible
   to the admin, and the UI has no "Pay now" to resume it. Designed but not built (decisions still to confirm):
   - "Pay now" on the Track Orders page via a code+phone-checked endpoint that returns the existing gateway order id
     (needs one new method on the `order.Payments` interface).
   - An `Idempotency-Key` on placing an order, with a unique index, so a double click or retry returns the same order.
   - A background sweeper in the server: every minute, reconcile pending orders older than ~2 min with Razorpay (mark paid if a
     payment landed), and cancel those older than the hold time (default 30 min) only after asking Razorpay again; use
     compare-and-set so it is safe with several instances.
   - A verified late payment may move `CANCELLED` back to `PLACED` (money is the stronger fact).
   - Defaults proposed: 30 min hold; reinstate (not auto-refund) on a late payment; idempotency key yes; sweeper in-process.
2. Refunds on reject are manual (Razorpay dashboard). Automatic refunds not built.
3. The Razorpay webhook endpoint exists but is off until `RAZORPAY_WEBHOOK_SECRET` is set (needs a public URL, e.g. ngrok).
4. No rate limiting on the public endpoints. SQL layers have no automated tests (only unit tests with fakes + manual runs).
5. Notifications ("order ready"), deleting `legacy/`, merging the PRs, rotating the secrets that leaked from the old Java repo.
6. **Order-tracking security (parked, decide together with item 1).** Tracking uses canteen + 6-char pickup code (31-char alphabet, about 887M
   combinations) + phone, and the browser remembers `{code, phone}` as `lastOrder`. The real gap is no rate limiting. Proposed: Part 1 per-IP rate
   limit on `/orders/track` (about 10/min, in-memory middleware in `httpx`; needs the real client IP behind a proxy); Part 2 a per-order random
   token (SHA-256 hash in a new nullable `orders.access_token_hash`, returned once at place-order, sent as `X-Order-Token`, code+phone kept as
   fallback). Recommendation: Part 1 now, Part 2 together with "Pay now". No signup or OTP (too slow for scan-and-order).
7. Branding gaps: the logo is a link only (no upload); no contrast check for very light accent colours; the Razorpay popup name and colour follow
   the branding but the Campus Bites logo file and orange remain the defaults.
8. The Your Bite website needs real content (team, services, story, contact), images, a repo and hosting.

## Working agreement with the owner
- They want this **engineered, not vibe-coded**: for anything non-trivial, propose the design and the trade-offs first, get a
  yes, then build. Give a recommendation with defaults, not a survey of options.
- Explain in plain language what changed and what was and was not verified. Say clearly when something was only built, not run.
- Be honest about uncertainty (for example the Razorpay test-card numbers were wrong once; verify instead of guessing).
- Commit and push only when asked; use a feature branch, never push straight to `main`. Never commit `.env`, secrets or large images.

## Local environment (Windows)
- Shell is PowerShell: `<` redirection does not work (pipe with `Get-Content file | mysql ...`), use `curl.exe` not `curl`.
- MySQL 8.0 runs as the `MySQL80` service on `localhost:3306`; `mysql.exe` is not on PATH:
  `C:\Program Files\MySQL\MySQL Server 8.0\bin\mysql.exe`. Dev database: `campus_bites_dev`. (`campus_bites` is the old prototype's DB.)
- Backend on `http://localhost:8081`, UI on `http://localhost:3000`.
- Razorpay is in **test mode** (`rzp_test_` keys). The India account rejects the common `4111 1111 1111 1111` card as
  "international". Test payments with UPI `success@razorpay` (and `failure@razorpay`) or a domestic test card from Razorpay's docs.
- Admin login for local dev: username `admin`, password is `ADMIN_PASSWORD` in `CampusBitesBackend\.env`.
- The dev database contains test orders (one created by the assistant with a simulated payment `pay_fake1`).
