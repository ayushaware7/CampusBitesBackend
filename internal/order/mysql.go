package order

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

const orderCols = `id, canteen_id, code, status, customer_name, customer_phone, customer_email, total_paise, created_at`

func scanOrder(row interface{ Scan(...any) error }) (Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.CanteenID, &o.Code, &o.Status, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.TotalPaise, &o.CreatedAt)
	o.Items = []Line{}
	return o, err
}

func (r *MySQLRepository) Create(ctx context.Context, o NewOrder) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `INSERT INTO orders (canteen_id, code, customer_name, customer_phone, customer_email, status, total_paise)
		VALUES (?,?,?,?,?,?,?)`, o.CanteenID, o.Code, o.Name, o.Phone, o.Email, PendingPayment, o.TotalPaise)
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return 0, errDuplicateCode
	}
	if err != nil {
		return 0, err
	}
	orderID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, l := range o.Lines {
		res, err := tx.ExecContext(ctx, `INSERT INTO order_items (order_id, item_id, item_name, variant_id, variant_name, quantity, unit_price_paise)
			VALUES (?,?,?,?,?,?,?)`, orderID, l.ItemID, l.ItemName, l.VariantID, l.VariantName, l.Quantity, l.UnitPricePaise)
		if err != nil {
			return 0, err
		}
		lineID, err := res.LastInsertId()
		if err != nil {
			return 0, err
		}
		for _, a := range l.Addons {
			if _, err := tx.ExecContext(ctx, `INSERT INTO order_item_addons (order_item_id, addon_id, addon_name, price_paise) VALUES (?,?,?,?)`,
				lineID, a.AddonID, a.Name, a.PricePaise); err != nil {
				return 0, err
			}
		}
	}
	return orderID, tx.Commit()
}

func (r *MySQLRepository) ByID(ctx context.Context, id int64) (*Order, error) {
	return r.one(ctx, `SELECT `+orderCols+` FROM orders WHERE id=?`, id)
}

func (r *MySQLRepository) ByCodeAndPhone(ctx context.Context, canteenID int64, code, phone string) (*Order, error) {
	return r.one(ctx, `SELECT `+orderCols+` FROM orders WHERE canteen_id=? AND code=? AND customer_phone=?`, canteenID, code, phone)
}

func (r *MySQLRepository) one(ctx context.Context, query string, args ...any) (*Order, error) {
	o, err := scanOrder(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("order not found")
	}
	if err != nil {
		return nil, err
	}
	orders := []Order{o}
	if err := r.attachLines(ctx, orders); err != nil {
		return nil, err
	}
	return &orders[0], nil
}

func (r *MySQLRepository) List(ctx context.Context, canteenID int64, statuses []Status, newestFirst bool) ([]Order, error) {
	args := []any{canteenID}
	for _, s := range statuses {
		args = append(args, s)
	}
	dir := "ASC" // live orders: oldest first, the kitchen works first-in first-out
	if newestFirst {
		dir = "DESC"
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+orderCols+` FROM orders WHERE canteen_id=? AND status IN (`+
		strings.TrimSuffix(strings.Repeat("?,", len(statuses)), ",")+`) ORDER BY created_at `+dir+`, id `+dir+` LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := []Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, r.attachLines(ctx, orders)
}

// attachLines fills Items for the given orders in two queries (lines, then their add-ons).
func (r *MySQLRepository) attachLines(ctx context.Context, orders []Order) error {
	if len(orders) == 0 {
		return nil
	}
	index := map[int64]int{} // order id -> position in orders
	args := make([]any, 0, len(orders))
	for i, o := range orders {
		index[o.ID] = i
		args = append(args, o.ID)
	}
	in := strings.TrimSuffix(strings.Repeat("?,", len(orders)), ",")

	rows, err := r.db.QueryContext(ctx, `SELECT order_id, id, item_id, item_name, variant_id, variant_name, quantity, unit_price_paise
		FROM order_items WHERE order_id IN (`+in+`) ORDER BY id`, args...)
	if err != nil {
		return err
	}
	type pos struct{ order, line int }
	where := map[int64]pos{} // order_items.id -> where that line sits
	for rows.Next() {
		var orderID, lineID int64
		l := Line{Addons: []AddonLine{}}
		if err := rows.Scan(&orderID, &lineID, &l.ItemID, &l.ItemName, &l.VariantID, &l.VariantName, &l.Quantity, &l.UnitPricePaise); err != nil {
			rows.Close()
			return err
		}
		i := index[orderID]
		where[lineID] = pos{i, len(orders[i].Items)}
		orders[i].Items = append(orders[i].Items, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	arows, err := r.db.QueryContext(ctx, `SELECT a.order_item_id, a.addon_id, a.addon_name, a.price_paise
		FROM order_item_addons a JOIN order_items i ON i.id=a.order_item_id
		WHERE i.order_id IN (`+in+`) ORDER BY a.id`, args...)
	if err != nil {
		return err
	}
	defer arows.Close()
	for arows.Next() {
		var lineID int64
		var a AddonLine
		if err := arows.Scan(&lineID, &a.AddonID, &a.Name, &a.PricePaise); err != nil {
			return err
		}
		p := where[lineID]
		orders[p.order].Items[p.line].Addons = append(orders[p.order].Items[p.line].Addons, a)
	}
	return arows.Err()
}

func (r *MySQLRepository) Transition(ctx context.Context, canteenID, id int64, from, to Status) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE orders SET status=? WHERE id=? AND canteen_id=? AND status=?`, to, id, canteenID, from)
	return affected(res, err)
}

func (r *MySQLRepository) AdvanceSystem(ctx context.Context, id int64, from, to Status) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE orders SET status=? WHERE id=? AND status=?`, to, id, from)
	return affected(res, err)
}

func affected(res sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
