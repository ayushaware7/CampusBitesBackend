-- Per-client branding shown in the customer app. All optional: NULL means "use the app default".
ALTER TABLE canteens
  ADD COLUMN tagline       VARCHAR(200) NULL,
  ADD COLUMN logo_url      VARCHAR(500) NULL,
  ADD COLUMN primary_color CHAR(7)      NULL;  -- '#rrggbb'
