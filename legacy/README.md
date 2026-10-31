# Legacy (first version)

The original flat implementation: auth, orders and Razorpay payment on the old schema (`schema.sql`).
Kept only as reference while those parts are ported into `internal/` modules. Not part of the new architecture; delete when the port is done.
Needs its own env vars (`RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET`, ...) and its own database.
