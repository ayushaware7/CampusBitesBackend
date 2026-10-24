-- Development data only. Run once on a freshly migrated database:
--   mysql -u root -p campus_bites_dev < db/seed_dev.sql
-- Prices are in paise (10000 = Rs 100).

INSERT INTO canteens (name, slug) VALUES ('Main Canteen', 'main-canteen');
SET @c = LAST_INSERT_ID();

INSERT INTO categories (canteen_id, name, sort_order) VALUES (@c, 'Breakfast', 1), (@c, 'Snacks', 2), (@c, 'Beverages', 3);
SET @breakfast = (SELECT id FROM categories WHERE canteen_id=@c AND name='Breakfast');
SET @snacks    = (SELECT id FROM categories WHERE canteen_id=@c AND name='Snacks');
SET @bev       = (SELECT id FROM categories WHERE canteen_id=@c AND name='Beverages');

INSERT INTO addon_groups (canteen_id, name, min_select, max_select, sort_order) VALUES
  (@c, 'Toppings', 0, 2, 1),
  (@c, 'Choose a sauce', 1, 1, 2);
SET @toppings = (SELECT id FROM addon_groups WHERE canteen_id=@c AND name='Toppings');
SET @sauce    = (SELECT id FROM addon_groups WHERE canteen_id=@c AND name='Choose a sauce');

INSERT INTO addons (group_id, name, price_paise, sort_order) VALUES
  (@toppings, 'Extra cheese', 2000, 1),
  (@toppings, 'Olives', 1500, 2),
  (@sauce, 'Ketchup', 0, 1),
  (@sauce, 'Mayo', 0, 2);

INSERT INTO items (canteen_id, category_id, name, description, price_paise, is_veg, is_special, sort_order) VALUES
  (@c, @breakfast, 'Poha', 'Flattened rice with peanuts', 4000, TRUE, FALSE, 1),
  (@c, @snacks, 'Veg Sandwich', 'Grilled, with chutney', 6000, TRUE, TRUE, 1),
  (@c, @snacks, 'Pizza', 'Hand tossed', 0, TRUE, FALSE, 2),
  (@c, @bev, 'Cold Coffee', 'Thick and sweet', 5000, TRUE, FALSE, 1);

SET @pizza = (SELECT id FROM items WHERE canteen_id=@c AND name='Pizza');
SET @sandwich = (SELECT id FROM items WHERE canteen_id=@c AND name='Veg Sandwich');
INSERT INTO item_variants (item_id, name, price_paise, sort_order) VALUES
  (@pizza, 'Small', 10000, 1),
  (@pizza, 'Large', 20000, 2);
INSERT INTO item_addon_groups (item_id, group_id) VALUES
  (@pizza, @toppings), (@pizza, @sauce), (@sandwich, @sauce);
