-- Payment module. order_id is a plain reference, deliberately without a foreign key: payments
-- may become a separate service with its own database.

CREATE TABLE payments (
  id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  order_id            BIGINT UNSIGNED NOT NULL,
  provider            VARCHAR(20)  NOT NULL DEFAULT 'razorpay',
  provider_order_id   VARCHAR(64)  NOT NULL,
  provider_payment_id VARCHAR(64)  NULL,       -- set once the customer has paid
  amount_paise        BIGINT       NOT NULL,
  status              VARCHAR(20)  NOT NULL,   -- CREATED PAID
  created_at          TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  paid_at             TIMESTAMP    NULL,
  UNIQUE KEY uq_payments_order (order_id),
  UNIQUE KEY uq_payments_provider_order (provider_order_id),
  UNIQUE KEY uq_payments_provider_payment (provider_payment_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
