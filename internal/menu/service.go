package menu

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

// CanteenLookup resolves a QR slug to a canteen id. Implemented by the canteen module.
type CanteenLookup interface {
	IDBySlug(ctx context.Context, slug string) (int64, error)
}

type Service struct {
	repo     Repository
	canteens CanteenLookup
}

func NewService(repo Repository, canteens CanteenLookup) *Service {
	return &Service{repo, canteens}
}

// ---------------------------------------------------------------- reads

func (s *Service) MenuBySlug(ctx context.Context, slug string) ([]Category, error) {
	id, err := s.canteens.IDBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.repo.Menu(ctx, id)
}

func (s *Service) Menu(ctx context.Context, canteenID int64) ([]Category, error) {
	return s.repo.Menu(ctx, canteenID)
}

func (s *Service) Item(ctx context.Context, canteenID, id int64) (*Item, error) {
	return s.repo.Item(ctx, canteenID, id)
}

func (s *Service) AddonGroups(ctx context.Context, canteenID int64) ([]AddonGroup, error) {
	return s.repo.AddonGroups(ctx, canteenID)
}

// ---------------------------------------------------------------- categories

func (s *Service) CreateCategory(ctx context.Context, canteenID int64, in CategoryInput) (*Category, error) {
	if err := validateCategory(&in); err != nil {
		return nil, err
	}
	id, err := s.repo.CreateCategory(ctx, canteenID, in)
	if err != nil {
		return nil, err
	}
	return &Category{ID: id, Name: in.Name, SortOrder: in.SortOrder, Items: []Item{}}, nil
}

func (s *Service) UpdateCategory(ctx context.Context, canteenID, id int64, in CategoryInput) (*Category, error) {
	if err := validateCategory(&in); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateCategory(ctx, canteenID, id, in); err != nil {
		return nil, err
	}
	return &Category{ID: id, Name: in.Name, SortOrder: in.SortOrder, Items: []Item{}}, nil
}

func (s *Service) DeleteCategory(ctx context.Context, canteenID, id int64) error {
	return s.repo.DeactivateCategory(ctx, canteenID, id)
}

// ---------------------------------------------------------------- items

// SaveItem creates (id 0) or updates an item and returns it as stored.
func (s *Service) SaveItem(ctx context.Context, canteenID, id int64, in ItemInput) (*Item, error) {
	if err := validateItem(&in); err != nil {
		return nil, err
	}
	id, err := s.repo.SaveItem(ctx, canteenID, id, in)
	if err != nil {
		return nil, err
	}
	return s.repo.Item(ctx, canteenID, id)
}

func (s *Service) SetItemAvailability(ctx context.Context, canteenID, id int64, available bool) error {
	return s.repo.SetItemAvailability(ctx, canteenID, id, available)
}

func (s *Service) DeleteItem(ctx context.Context, canteenID, id int64) error {
	return s.repo.DeactivateItem(ctx, canteenID, id)
}

// ---------------------------------------------------------------- add-on groups

func (s *Service) SaveAddonGroup(ctx context.Context, canteenID, id int64, in AddonGroupInput) (*AddonGroup, error) {
	if err := validateGroup(&in); err != nil {
		return nil, err
	}
	id, err := s.repo.SaveAddonGroup(ctx, canteenID, id, in)
	if err != nil {
		return nil, err
	}
	groups, err := s.repo.AddonGroups(ctx, canteenID)
	if err != nil {
		return nil, err
	}
	for i := range groups {
		if groups[i].ID == id {
			return &groups[i], nil
		}
	}
	return nil, apperr.NotFound("add-on group not found")
}

func (s *Service) DeleteAddonGroup(ctx context.Context, canteenID, id int64) error {
	return s.repo.DeactivateAddonGroup(ctx, canteenID, id)
}

// ---------------------------------------------------------------- validation

func cleanName(field, v string, max int) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", apperr.Invalid("%s is required", field)
	}
	if utf8.RuneCountInString(v) > max {
		return "", apperr.Invalid("%s must be at most %d characters", field, max)
	}
	return v, nil
}

func validateCategory(in *CategoryInput) (err error) {
	in.Name, err = cleanName("name", in.Name, 100)
	return err
}

func validateItem(in *ItemInput) (err error) {
	if in.CategoryID <= 0 {
		return apperr.Invalid("categoryId is required")
	}
	if in.Name, err = cleanName("name", in.Name, 100); err != nil {
		return err
	}
	in.Description = strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(in.Description) > 500 {
		return apperr.Invalid("description must be at most 500 characters")
	}
	in.ImageURL = strings.TrimSpace(in.ImageURL)
	if len(in.ImageURL) > 255 {
		return apperr.Invalid("imageUrl must be at most 255 characters")
	}
	if in.ImageURL != "" && !strings.HasPrefix(in.ImageURL, "https://") && !strings.HasPrefix(in.ImageURL, "http://") && !strings.HasPrefix(in.ImageURL, "/") {
		return apperr.Invalid("imageUrl must be an http(s) URL or a path starting with /")
	}

	// Without variants the base price is the price, so it must be real; with variants it is unused.
	if len(in.Variants) == 0 && in.PricePaise <= 0 {
		return apperr.Invalid("pricePaise must be greater than 0")
	}
	if in.PricePaise < 0 {
		return apperr.Invalid("pricePaise cannot be negative")
	}
	seen := map[string]bool{}
	for i := range in.Variants {
		v := &in.Variants[i]
		if v.Name, err = cleanName("variant name", v.Name, 50); err != nil {
			return err
		}
		if v.PricePaise <= 0 {
			return apperr.Invalid("variant %q: pricePaise must be greater than 0", v.Name)
		}
		key := strings.ToLower(v.Name)
		if seen[key] {
			return apperr.Invalid("variant %q is listed twice", v.Name)
		}
		seen[key] = true
	}

	ids := map[int64]bool{}
	for _, g := range in.AddonGroupIDs {
		if g <= 0 || ids[g] {
			return apperr.Invalid("addonGroupIds must be unique positive ids")
		}
		ids[g] = true
	}
	return nil
}

func validateGroup(in *AddonGroupInput) (err error) {
	if in.Name, err = cleanName("name", in.Name, 100); err != nil {
		return err
	}
	if in.MaxSelect < 1 || in.MinSelect < 0 || in.MinSelect > in.MaxSelect {
		return apperr.Invalid("need 0 <= minSelect <= maxSelect and maxSelect >= 1")
	}
	if in.MinSelect > len(in.Addons) {
		return apperr.Invalid("minSelect %d is more than the %d add-ons offered", in.MinSelect, len(in.Addons))
	}
	seen := map[string]bool{}
	for i := range in.Addons {
		a := &in.Addons[i]
		if a.Name, err = cleanName("add-on name", a.Name, 50); err != nil {
			return err
		}
		if a.PricePaise < 0 {
			return apperr.Invalid("add-on %q: pricePaise cannot be negative", a.Name)
		}
		key := strings.ToLower(a.Name)
		if seen[key] {
			return apperr.Invalid("add-on %q is listed twice", a.Name)
		}
		seen[key] = true
	}
	return nil
}
