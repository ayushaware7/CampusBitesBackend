CREATE DATABASE IF NOT EXISTS campus_bites;
USE campus_bites;

CREATE TABLE IF NOT EXISTS canteens (
  id   BIGINT AUTO_INCREMENT PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  slug VARCHAR(50)  NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS items (
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  canteen_id BIGINT NOT NULL,
  name       VARCHAR(100) NOT NULL,
  price      DECIMAL(10,2) NOT NULL,
  rating     FLOAT NOT NULL DEFAULT 0,
  image      VARCHAR(255) NOT NULL DEFAULT '',
  category   VARCHAR(50)  NOT NULL DEFAULT '',
  FOREIGN KEY (canteen_id) REFERENCES canteens(id)
);
CREATE TABLE IF NOT EXISTS admins (
  id            BIGINT AUTO_INCREMENT PRIMARY KEY,
  username      VARCHAR(50)  NOT NULL UNIQUE,
  password_hash VARCHAR(100) NOT NULL,
  canteen_id    BIGINT NOT NULL,
  FOREIGN KEY (canteen_id) REFERENCES canteens(id)
);

-- paid orders waiting to be prepared; the admin moves them to past_orders
CREATE TABLE IF NOT EXISTS live_orders (
  id                  BIGINT AUTO_INCREMENT PRIMARY KEY,
  canteen_id          BIGINT NOT NULL,
  razorpay_order_id   VARCHAR(64) NOT NULL,
  razorpay_payment_id VARCHAR(64) NOT NULL UNIQUE,
  customer_name       VARCHAR(100) NOT NULL,
  customer_email      VARCHAR(100) NOT NULL DEFAULT '',
  customer_phone      VARCHAR(20)  NOT NULL,
  total_amount        DECIMAL(10,2) NOT NULL,
  order_date          DATETIME NOT NULL,
  items_json          TEXT NOT NULL,
  INDEX (canteen_id, customer_phone),
  FOREIGN KEY (canteen_id) REFERENCES canteens(id)
);

CREATE TABLE IF NOT EXISTS past_orders (
  id                  BIGINT AUTO_INCREMENT PRIMARY KEY,
  canteen_id          BIGINT NOT NULL,
  original_order_id   BIGINT NOT NULL,
  razorpay_order_id   VARCHAR(64) NOT NULL,
  razorpay_payment_id VARCHAR(64) NOT NULL,
  customer_name       VARCHAR(100) NOT NULL,
  customer_email      VARCHAR(100) NOT NULL DEFAULT '',
  customer_phone      VARCHAR(20)  NOT NULL,
  total_amount        DECIMAL(10,2) NOT NULL,
  order_date          DATETIME NOT NULL,
  items_json          TEXT NOT NULL,
  status              VARCHAR(20) NOT NULL,
  completed_date      DATETIME NOT NULL,
  INDEX (canteen_id, customer_phone),
  FOREIGN KEY (canteen_id) REFERENCES canteens(id)
);

-- sample canteen; the QR for it points at /c/main-canteen
INSERT IGNORE INTO canteens (name, slug) VALUES ('Main Canteen', 'main-canteen');
