package menu

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(db *sql.DB) *MySQLRepository { return &MySQLRepository{db} }

// dbtx is satisfied by both *sql.DB and *sql.Tx.
type dbtx interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func isDuplicate(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// requireOne turns "no row matched" into NotFound. Relies on clientFoundRows (see database.Open).
func requireOne(res sql.Result, what string) error {
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.NotFound("%s not found", what)
	}
	return nil
}

func placeholders(n int) string { return strings.TrimSuffix(strings.Repeat("?,", n), ",") }

func queryIDs(ctx context.Context, q dbtx, query string, args ...any) (map[int64]bool, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------- reads

func (r *MySQLRepository) Menu(ctx context.Context, canteenID int64) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, sort_order FROM categories WHERE canteen_id=? AND is_active=1 ORDER BY sort_order, id`, canteenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cats := []Category{}
	index := map[int64]int{}
	for rows.Next() {
		c := Category{Items: []Item{}}
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder); err != nil {
			return nil, err
		}
		index[c.ID] = len(cats)
		cats = append(cats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items, err := loadItems(ctx, r.db, canteenID, 0)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if i, ok := index[it.CategoryID]; ok {
			cats[i].Items = append(cats[i].Items, it)
		}
	}
	return cats, nil
}

func (r *MySQLRepository) Item(ctx context.Context, canteenID, itemID int64) (*Item, error) {
	items, err := loadItems(ctx, r.db, canteenID, itemID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, apperr.NotFound("item not found")
	}
	return &items[0], nil
}

// loadItems reads active items with their variants and add-on groups in four queries.
// itemID 0 means every item of the canteen.
func loadItems(ctx context.Context, q dbtx, canteenID, itemID int64) ([]Item, error) {
	filter, args := "", []any{canteenID}
	if itemID != 0 {
		filter, args = " AND i.id=?", append(args, itemID)
	}

	rows, err := q.QueryContext(ctx, `SELECT i.id, i.category_id, i.name, i.description, i.image_url, i.price_paise,
		i.is_veg, i.is_special, i.is_available, i.sort_order
		FROM items i WHERE i.canteen_id=? AND i.is_active=1`+filter+` ORDER BY i.sort_order, i.id`, args...)
	if err != nil {
		return nil, err
	}
	items := []Item{}
	index := map[int64]int{}
	for rows.Next() {
		it := Item{Variants: []Variant{}, AddonGroups: []AddonGroup{}}
		if err := rows.Scan(&it.ID, &it.CategoryID, &it.Name, &it.Description, &it.ImageURL, &it.PricePaise,
			&it.IsVeg, &it.IsSpecial, &it.IsAvailable, &it.SortOrder); err != nil {
			rows.Close()
			return nil, err
		}
		index[it.ID] = len(items)
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(items) == 0 {
		return items, err
	}

	vrows, err := q.QueryContext(ctx, `SELECT v.item_id, v.id, v.name, v.price_paise, v.is_available, v.sort_order
		FROM item_variants v JOIN items i ON i.id=v.item_id
		WHERE i.canteen_id=? AND i.is_active=1 AND v.is_active=1`+filter+` ORDER BY v.sort_order, v.id`, args...)
	if err != nil {
		return nil, err
	}
	for vrows.Next() {
		var itemID int64
		var v Variant
		if err := vrows.Scan(&itemID, &v.ID, &v.Name, &v.PricePaise, &v.IsAvailable, &v.SortOrder); err != nil {
			vrows.Close()
			return nil, err
		}
		items[index[itemID]].Variants = append(items[index[itemID]].Variants, v)
	}
	vrows.Close()
	if err := vrows.Err(); err != nil {
		return nil, err
	}

	addons, err := loadAddons(ctx, q, canteenID)
	if err != nil {
		return nil, err
	}
	grows, err := q.QueryContext(ctx, `SELECT l.item_id, g.id, g.name, g.min_select, g.max_select, g.sort_order
		FROM item_addon_groups l
		JOIN addon_groups g ON g.id=l.group_id
		JOIN items i ON i.id=l.item_id
		WHERE i.canteen_id=? AND i.is_active=1 AND g.is_active=1`+filter+` ORDER BY g.sort_order, g.id`, args...)
	if err != nil {
		return nil, err
	}
	defer grows.Close()
	for grows.Next() {
		var itemID int64
		g := AddonGroup{}
		if err := grows.Scan(&itemID, &g.ID, &g.Name, &g.MinSelect, &g.MaxSelect, &g.SortOrder); err != nil {
			return nil, err
		}
		g.Addons = addons[g.ID]
		if g.Addons == nil {
			g.Addons = []Addon{}
		}
		items[index[itemID]].AddonGroups = append(items[index[itemID]].AddonGroups, g)
	}
	return items, grows.Err()
}

// loadAddons returns the active add-ons of a canteen's active groups, keyed by group id.
func loadAddons(ctx context.Context, q dbtx, canteenID int64) (map[int64][]Addon, error) {
	rows, err := q.QueryContext(ctx, `SELECT a.group_id, a.id, a.name, a.price_paise, a.is_available, a.sort_order
		FROM addons a JOIN addon_groups g ON g.id=a.group_id
		WHERE g.canteen_id=? AND g.is_active=1 AND a.is_active=1 ORDER BY a.sort_order, a.id`, canteenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]Addon{}
	for rows.Next() {
		var gid int64
		var a Addon
		if err := rows.Scan(&gid, &a.ID, &a.Name, &a.PricePaise, &a.IsAvailable, &a.SortOrder); err != nil {
			return nil, err
		}
		out[gid] = append(out[gid], a)
	}
	return out, rows.Err()
}

func (r *MySQLRepository) AddonGroups(ctx context.Context, canteenID int64) ([]AddonGroup, error) {
	addons, err := loadAddons(ctx, r.db, canteenID)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, min_select, max_select, sort_order
		FROM addon_groups WHERE canteen_id=? AND is_active=1 ORDER BY sort_order, id`, canteenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []AddonGroup{}
	for rows.Next() {
		var g AddonGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.MinSelect, &g.MaxSelect, &g.SortOrder); err != nil {
			return nil, err
		}
		g.Addons = addons[g.ID]
		if g.Addons == nil {
			g.Addons = []Addon{}
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// ---------------------------------------------------------------- categories

func (r *MySQLRepository) CreateCategory(ctx context.Context, canteenID int64, in CategoryInput) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO categories (canteen_id, name, sort_order) VALUES (?,?,?)`,
		canteenID, in.Name, in.SortOrder)
	if isDuplicate(err) {
		return 0, apperr.Invalid("a category named %q already exists", in.Name)
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *MySQLRepository) UpdateCategory(ctx context.Context, canteenID, id int64, in CategoryInput) error {
	res, err := r.db.ExecContext(ctx, `UPDATE categories SET name=?, sort_order=? WHERE id=? AND canteen_id=? AND is_active=1`,
		in.Name, in.SortOrder, id, canteenID)
	if isDuplicate(err) {
		return apperr.Invalid("a category named %q already exists", in.Name)
	}
	if err != nil {
		return err
	}
	return requireOne(res, "category")
}

func (r *MySQLRepository) DeactivateCategory(ctx context.Context, canteenID, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE category_id=? AND canteen_id=? AND is_active=1`, id, canteenID).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return apperr.Invalid("category still has %d item(s); move or delete them first", n)
	}
	res, err := tx.ExecContext(ctx, `UPDATE categories SET is_active=0 WHERE id=? AND canteen_id=? AND is_active=1`, id, canteenID)
	if err != nil {
		return err
	}
	if err := requireOne(res, "category"); err != nil {
		return err
	}
	return tx.Commit()
}

// ---------------------------------------------------------------- items

func (r *MySQLRepository) SaveItem(ctx context.Context, canteenID, id int64, in ItemInput) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var one int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM categories WHERE id=? AND canteen_id=? AND is_active=1`, in.CategoryID, canteenID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, apperr.Invalid("category not found")
	}
	if err != nil {
		return 0, err
	}

	if id == 0 {
		res, err := tx.ExecContext(ctx, `INSERT INTO items
			(canteen_id, category_id, name, description, image_url, price_paise, is_veg, is_special, is_available, sort_order)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			canteenID, in.CategoryID, in.Name, in.Description, in.ImageURL, in.PricePaise, in.IsVeg, in.IsSpecial, !in.SoldOut, in.SortOrder)
		if err != nil {
			return 0, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return 0, err
		}
	} else {
		res, err := tx.ExecContext(ctx, `UPDATE items SET category_id=?, name=?, description=?, image_url=?, price_paise=?,
			is_veg=?, is_special=?, is_available=?, sort_order=? WHERE id=? AND canteen_id=? AND is_active=1`,
			in.CategoryID, in.Name, in.Description, in.ImageURL, in.PricePaise, in.IsVeg, in.IsSpecial, !in.SoldOut, in.SortOrder, id, canteenID)
		if err != nil {
			return 0, err
		}
		if err := requireOne(res, "item"); err != nil {
			return 0, err
		}
	}

	if err := syncVariants(ctx, tx, id, in.Variants); err != nil {
		return 0, err
	}
	if err := linkGroups(ctx, tx, canteenID, id, in.AddonGroupIDs); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// syncVariants upserts the given variants and soft-deletes the ones no longer listed,
// keeping ids stable so carts holding a variant id keep working across edits.
func syncVariants(ctx context.Context, tx *sql.Tx, itemID int64, in []VariantInput) error {
	existing, err := queryIDs(ctx, tx, `SELECT id FROM item_variants WHERE item_id=? AND is_active=1`, itemID)
	if err != nil {
		return err
	}
	keep := map[int64]bool{}
	for _, v := range in {
		if v.ID == 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO item_variants (item_id, name, price_paise, is_available, sort_order) VALUES (?,?,?,?,?)`,
				itemID, v.Name, v.PricePaise, !v.SoldOut, v.SortOrder); err != nil {
				return err
			}
			continue
		}
		if !existing[v.ID] {
			return apperr.Invalid("variant %d does not belong to this item", v.ID)
		}
		keep[v.ID] = true
		if _, err := tx.ExecContext(ctx, `UPDATE item_variants SET name=?, price_paise=?, is_available=?, sort_order=? WHERE id=? AND item_id=?`,
			v.Name, v.PricePaise, !v.SoldOut, v.SortOrder, v.ID, itemID); err != nil {
			return err
		}
	}
	for id := range existing {
		if !keep[id] {
			if _, err := tx.ExecContext(ctx, `UPDATE item_variants SET is_active=0 WHERE id=?`, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// linkGroups replaces the item's add-on group links after checking every group belongs to the canteen.
func linkGroups(ctx context.Context, tx *sql.Tx, canteenID, itemID int64, groupIDs []int64) error {
	if len(groupIDs) > 0 {
		args := []any{canteenID}
		for _, g := range groupIDs {
			args = append(args, g)
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM addon_groups WHERE canteen_id=? AND is_active=1 AND id IN (`+placeholders(len(groupIDs))+`)`, args...).Scan(&n); err != nil {
			return err
		}
		if n != len(groupIDs) {
			return apperr.Invalid("unknown add-on group")
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_addon_groups WHERE item_id=?`, itemID); err != nil {
		return err
	}
	for _, g := range groupIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO item_addon_groups (item_id, group_id) VALUES (?,?)`, itemID, g); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) SetItemAvailability(ctx context.Context, canteenID, id int64, available bool) error {
	res, err := r.db.ExecContext(ctx, `UPDATE items SET is_available=? WHERE id=? AND canteen_id=? AND is_active=1`, available, id, canteenID)
	if err != nil {
		return err
	}
	return requireOne(res, "item")
}

func (r *MySQLRepository) DeactivateItem(ctx context.Context, canteenID, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE items SET is_active=0 WHERE id=? AND canteen_id=? AND is_active=1`, id, canteenID)
	if err != nil {
		return err
	}
	return requireOne(res, "item")
}

// ---------------------------------------------------------------- add-on groups

func (r *MySQLRepository) SaveAddonGroup(ctx context.Context, canteenID, id int64, in AddonGroupInput) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if id == 0 {
		res, err := tx.ExecContext(ctx, `INSERT INTO addon_groups (canteen_id, name, min_select, max_select, sort_order) VALUES (?,?,?,?,?)`,
			canteenID, in.Name, in.MinSelect, in.MaxSelect, in.SortOrder)
		if err != nil {
			return 0, err
		}
		if id, err = res.LastInsertId(); err != nil {
			return 0, err
		}
	} else {
		res, err := tx.ExecContext(ctx, `UPDATE addon_groups SET name=?, min_select=?, max_select=?, sort_order=? WHERE id=? AND canteen_id=? AND is_active=1`,
			in.Name, in.MinSelect, in.MaxSelect, in.SortOrder, id, canteenID)
		if err != nil {
			return 0, err
		}
		if err := requireOne(res, "add-on group"); err != nil {
			return 0, err
		}
	}

	existing, err := queryIDs(ctx, tx, `SELECT id FROM addons WHERE group_id=? AND is_active=1`, id)
	if err != nil {
		return 0, err
	}
	keep := map[int64]bool{}
	for _, a := range in.Addons {
		if a.ID == 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO addons (group_id, name, price_paise, is_available, sort_order) VALUES (?,?,?,?,?)`,
				id, a.Name, a.PricePaise, !a.SoldOut, a.SortOrder); err != nil {
				return 0, err
			}
			continue
		}
		if !existing[a.ID] {
			return 0, apperr.Invalid("add-on %d does not belong to this group", a.ID)
		}
		keep[a.ID] = true
		if _, err := tx.ExecContext(ctx, `UPDATE addons SET name=?, price_paise=?, is_available=?, sort_order=? WHERE id=? AND group_id=?`,
			a.Name, a.PricePaise, !a.SoldOut, a.SortOrder, a.ID, id); err != nil {
			return 0, err
		}
	}
	for aid := range existing {
		if !keep[aid] {
			if _, err := tx.ExecContext(ctx, `UPDATE addons SET is_active=0 WHERE id=?`, aid); err != nil {
				return 0, err
			}
		}
	}
	return id, tx.Commit()
}

func (r *MySQLRepository) DeactivateAddonGroup(ctx context.Context, canteenID, id int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE addon_groups SET is_active=0 WHERE id=? AND canteen_id=? AND is_active=1`, id, canteenID)
	if err != nil {
		return err
	}
	return requireOne(res, "add-on group")
}
