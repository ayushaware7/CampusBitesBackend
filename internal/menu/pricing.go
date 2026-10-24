package menu

import (
	"context"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

// LineRequest is what a customer picked for one cart line. Only ids come from the client.
type LineRequest struct {
	ItemID    int64
	VariantID int64 // 0 when the item has no variants
	AddonIDs  []int64
}

type PricedAddon struct {
	ID         int64
	Name       string
	PricePaise int64
}

// PricedLine is the server-calculated result, with the names an order should snapshot.
type PricedLine struct {
	ItemID         int64
	ItemName       string
	VariantID      int64
	VariantName    string
	Addons         []PricedAddon
	UnitPricePaise int64 // variant (or base) price plus add-ons, for quantity 1
}

// PriceLine validates one cart line against the live menu and prices it.
// The order module calls this; it never reads menu tables itself.
func (s *Service) PriceLine(ctx context.Context, canteenID int64, req LineRequest) (PricedLine, error) {
	item, err := s.repo.Item(ctx, canteenID, req.ItemID)
	if kind, ok := apperr.KindOf(err); ok && kind == apperr.KindNotFound {
		return PricedLine{}, apperr.Invalid("item %d is not on this canteen's menu", req.ItemID)
	}
	if err != nil {
		return PricedLine{}, err
	}
	return priceLine(item, req)
}

func priceLine(item *Item, req LineRequest) (PricedLine, error) {
	if !item.IsAvailable {
		return PricedLine{}, apperr.Invalid("%s is sold out", item.Name)
	}
	line := PricedLine{ItemID: item.ID, ItemName: item.Name, UnitPricePaise: item.PricePaise}

	if len(item.Variants) > 0 {
		if req.VariantID == 0 {
			return PricedLine{}, apperr.Invalid("choose a variant for %s", item.Name)
		}
		var picked *Variant
		for i := range item.Variants {
			if item.Variants[i].ID == req.VariantID {
				picked = &item.Variants[i]
			}
		}
		if picked == nil {
			return PricedLine{}, apperr.Invalid("unknown variant for %s", item.Name)
		}
		if !picked.IsAvailable {
			return PricedLine{}, apperr.Invalid("%s (%s) is sold out", item.Name, picked.Name)
		}
		line.VariantID, line.VariantName, line.UnitPricePaise = picked.ID, picked.Name, picked.PricePaise
	} else if req.VariantID != 0 {
		return PricedLine{}, apperr.Invalid("%s has no variants", item.Name)
	}

	chosen := map[int64]bool{}
	for _, id := range req.AddonIDs {
		if chosen[id] {
			return PricedLine{}, apperr.Invalid("add-on %d chosen more than once", id)
		}
		chosen[id] = true
	}
	for _, g := range item.AddonGroups {
		n := 0
		for _, a := range g.Addons {
			if !chosen[a.ID] {
				continue
			}
			if !a.IsAvailable {
				return PricedLine{}, apperr.Invalid("%s is sold out", a.Name)
			}
			n++
			delete(chosen, a.ID)
			line.Addons = append(line.Addons, PricedAddon{a.ID, a.Name, a.PricePaise})
			line.UnitPricePaise += a.PricePaise
		}
		if n < g.MinSelect || n > g.MaxSelect {
			if g.MinSelect == g.MaxSelect {
				return PricedLine{}, apperr.Invalid("choose exactly %d option(s) from %q", g.MinSelect, g.Name)
			}
			return PricedLine{}, apperr.Invalid("choose %d to %d option(s) from %q", g.MinSelect, g.MaxSelect, g.Name)
		}
	}
	if len(chosen) > 0 { // ids that matched no group of this item
		return PricedLine{}, apperr.Invalid("invalid add-on for %s", item.Name)
	}
	return line, nil
}
