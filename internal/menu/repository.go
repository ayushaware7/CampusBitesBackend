package menu

import "context"

// Repository is the storage contract; MySQLRepository implements it and tests use a fake.
// Every method is scoped by canteenID so one canteen can never touch another's rows.
type Repository interface {
	Menu(ctx context.Context, canteenID int64) ([]Category, error)
	Item(ctx context.Context, canteenID, itemID int64) (*Item, error)

	CreateCategory(ctx context.Context, canteenID int64, in CategoryInput) (int64, error)
	UpdateCategory(ctx context.Context, canteenID, id int64, in CategoryInput) error
	DeactivateCategory(ctx context.Context, canteenID, id int64) error

	// SaveItem creates the item when id is 0, otherwise updates it, together with its
	// variants and add-on group links, in one transaction.
	SaveItem(ctx context.Context, canteenID, id int64, in ItemInput) (int64, error)
	SetItemAvailability(ctx context.Context, canteenID, id int64, available bool) error
	DeactivateItem(ctx context.Context, canteenID, id int64) error

	AddonGroups(ctx context.Context, canteenID int64) ([]AddonGroup, error)
	SaveAddonGroup(ctx context.Context, canteenID, id int64, in AddonGroupInput) (int64, error)
	DeactivateAddonGroup(ctx context.Context, canteenID, id int64) error
}
