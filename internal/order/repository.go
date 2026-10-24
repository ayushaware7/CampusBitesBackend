package order

import (
	"context"
	"errors"
)

// errDuplicateCode means the generated pickup code is already used in that canteen; Place retries.
var errDuplicateCode = errors.New("duplicate order code")

type Repository interface {
	// Create stores the order and its lines in one transaction. Returns errDuplicateCode on a code clash.
	Create(ctx context.Context, o NewOrder) (int64, error)
	ByID(ctx context.Context, id int64) (*Order, error)
	ByCodeAndPhone(ctx context.Context, canteenID int64, code, phone string) (*Order, error)
	List(ctx context.Context, canteenID int64, statuses []Status, newestFirst bool) ([]Order, error)

	// Transition moves an order only if it is still in `from` (compare-and-set), scoped to the
	// canteen. It reports false when nothing matched, e.g. someone else changed it first.
	Transition(ctx context.Context, canteenID, id int64, from, to Status) (bool, error)
	// AdvanceSystem is the same compare-and-set without a canteen, for system-driven changes.
	AdvanceSystem(ctx context.Context, id int64, from, to Status) (bool, error)
}
