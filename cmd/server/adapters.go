package main

import (
	"context"

	"github.com/ayushaware7/CampusBitesBackend/internal/menu"
	"github.com/ayushaware7/CampusBitesBackend/internal/order"
)

// menuPricer lets the order module price cart lines without importing the menu module.
// If menu becomes its own service, only this adapter changes (to an HTTP/gRPC client).
type menuPricer struct{ svc *menu.Service }

func (p menuPricer) PriceLine(ctx context.Context, canteenID, itemID, variantID int64, addonIDs []int64) (order.PricedLine, error) {
	l, err := p.svc.PriceLine(ctx, canteenID, menu.LineRequest{ItemID: itemID, VariantID: variantID, AddonIDs: addonIDs})
	if err != nil {
		return order.PricedLine{}, err
	}
	out := order.PricedLine{
		ItemID: l.ItemID, ItemName: l.ItemName, VariantID: l.VariantID, VariantName: l.VariantName,
		UnitPricePaise: l.UnitPricePaise,
	}
	for _, a := range l.Addons {
		out.Addons = append(out.Addons, order.PricedAddon{ID: a.ID, Name: a.Name, PricePaise: a.PricePaise})
	}
	return out, nil
}
