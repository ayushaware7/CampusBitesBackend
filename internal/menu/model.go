// Package menu owns categories, items, variants and add-ons. Money is integer paise.
// Other modules (orders) must go through Service, never read these tables directly.
package menu

type Variant struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	PricePaise  int64  `json:"pricePaise"` // absolute price; replaces the item's base price
	IsAvailable bool   `json:"isAvailable"`
	SortOrder   int    `json:"sortOrder"`
}

type Addon struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	PricePaise  int64  `json:"pricePaise"` // added on top of the item price
	IsAvailable bool   `json:"isAvailable"`
	SortOrder   int    `json:"sortOrder"`
}

type AddonGroup struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	MinSelect int     `json:"minSelect"`
	MaxSelect int     `json:"maxSelect"`
	SortOrder int     `json:"sortOrder"`
	Addons    []Addon `json:"addons"`
}

type Item struct {
	ID          int64        `json:"id"`
	CategoryID  int64        `json:"categoryId"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	ImageURL    string       `json:"imageUrl"`
	PricePaise  int64        `json:"pricePaise"` // base price, used when Variants is empty
	IsVeg       bool         `json:"isVeg"`
	IsSpecial   bool         `json:"isSpecial"`
	IsAvailable bool         `json:"isAvailable"` // false = sold out today
	SortOrder   int          `json:"sortOrder"`
	Variants    []Variant    `json:"variants"`
	AddonGroups []AddonGroup `json:"addonGroups"`
}

type Category struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	Items     []Item `json:"items"`
}

// ---- admin inputs. "soldOut" is inverted so an omitted field means available. ----

type CategoryInput struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

// VariantInput with ID 0 is created; an existing ID is updated; omitted variants are removed.
type VariantInput struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	PricePaise int64  `json:"pricePaise"`
	SoldOut    bool   `json:"soldOut"`
	SortOrder  int    `json:"sortOrder"`
}

type ItemInput struct {
	CategoryID    int64          `json:"categoryId"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	ImageURL      string         `json:"imageUrl"`
	PricePaise    int64          `json:"pricePaise"`
	IsVeg         bool           `json:"isVeg"`
	IsSpecial     bool           `json:"isSpecial"`
	SoldOut       bool           `json:"soldOut"`
	SortOrder     int            `json:"sortOrder"`
	Variants      []VariantInput `json:"variants"`
	AddonGroupIDs []int64        `json:"addonGroupIds"`
}

type AddonInput struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	PricePaise int64  `json:"pricePaise"`
	SoldOut    bool   `json:"soldOut"`
	SortOrder  int    `json:"sortOrder"`
}

type AddonGroupInput struct {
	Name      string       `json:"name"`
	MinSelect int          `json:"minSelect"`
	MaxSelect int          `json:"maxSelect"`
	SortOrder int          `json:"sortOrder"`
	Addons    []AddonInput `json:"addons"`
}
