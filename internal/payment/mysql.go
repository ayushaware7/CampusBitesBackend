package payment

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(db *sql.DB) *MySQLRepository { return &MySQLRepository{db} }

func (r *MySQLRepository) Create(ctx context.Context, p Payment) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO payments (order_id, provider_order_id, amount_paise, status) VALUES (?,?,?,?)`,
		p.OrderID, p.ProviderOrderID, p.AmountPaise, p.Status)
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return apperr.Invalid("a payment already exists for this order")
	}
	return err
}

const paymentCols = `id, order_id, provider_order_id, COALESCE(provider_payment_id, ''), amount_paise, status`

func (r *MySQLRepository) one(ctx context.Context, where string, arg any) (*Payment, error) {
	var p Payment
	err := r.db.QueryRowContext(ctx, `SELECT `+paymentCols+` FROM payments WHERE `+where, arg).
		Scan(&p.ID, &p.OrderID, &p.ProviderOrderID, &p.ProviderPaymentID, &p.AmountPaise, &p.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("payment not found")
	}
	return &p, err
}

func (r *MySQLRepository) ByOrderID(ctx context.Context, orderID int64) (*Payment, error) {
	return r.one(ctx, `order_id=?`, orderID)
}

func (r *MySQLRepository) ByProviderOrderID(ctx context.Context, id string) (*Payment, error) {
	return r.one(ctx, `provider_order_id=?`, id)
}

func (r *MySQLRepository) MarkPaid(ctx context.Context, id int64, providerPaymentID string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE payments SET status=?, provider_payment_id=?, paid_at=COALESCE(paid_at, ?)
		WHERE id=? AND (provider_payment_id IS NULL OR provider_payment_id=?)`,
		StatusPaid, providerPaymentID, time.Now().UTC(), id, providerPaymentID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperr.Invalid("this order was already paid with a different payment")
	}
	return nil
}
