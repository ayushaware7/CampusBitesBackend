package menu

import (
	"strings"
	"testing"
)

// pizza: two sizes, a required crust choice (exactly 1) and optional toppings (0..2).
func pizza() *Item {
	return &Item{
		ID: 1, Name: "Pizza", PricePaise: 0, IsAvailable: true,
		Variants: []Variant{
			{ID: 10, Name: "Small", PricePaise: 10000, IsAvailable: true},
			{ID: 11, Name: "Large", PricePaise: 20000, IsAvailable: true},
			{ID: 12, Name: "Party", PricePaise: 30000, IsAvailable: false},
		},
		AddonGroups: []AddonGroup{
			{ID: 100, Name: "Crust", MinSelect: 1, MaxSelect: 1, Addons: []Addon{
				{ID: 1000, Name: "Thin", PricePaise: 0, IsAvailable: true},
				{ID: 1001, Name: "Cheese burst", PricePaise: 4000, IsAvailable: true},
			}},
			{ID: 101, Name: "Toppings", MinSelect: 0, MaxSelect: 2, Addons: []Addon{
				{ID: 1010, Name: "Olives", PricePaise: 1500, IsAvailable: true},
				{ID: 1011, Name: "Corn", PricePaise: 1000, IsAvailable: true},
				{ID: 1012, Name: "Jalapeno", PricePaise: 1200, IsAvailable: false},
			}},
		},
	}
}

func TestPriceLine(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Item)
		req     LineRequest
		want    int64  // unit price in paise when valid
		wantErr string // substring of the error when invalid
	}{
		{name: "variant plus required crust", req: LineRequest{ItemID: 1, VariantID: 11, AddonIDs: []int64{1000}}, want: 20000},
		{name: "variant, paid crust and toppings sum up", req: LineRequest{ItemID: 1, VariantID: 10, AddonIDs: []int64{1001, 1010, 1011}}, want: 10000 + 4000 + 1500 + 1000},
		{name: "variant required", req: LineRequest{ItemID: 1, AddonIDs: []int64{1000}}, wantErr: "choose a variant"},
		{name: "unknown variant", req: LineRequest{ItemID: 1, VariantID: 99, AddonIDs: []int64{1000}}, wantErr: "unknown variant"},
		{name: "sold-out variant", req: LineRequest{ItemID: 1, VariantID: 12, AddonIDs: []int64{1000}}, wantErr: "sold out"},
		{name: "required group missing", req: LineRequest{ItemID: 1, VariantID: 10}, wantErr: `from "Crust"`},
		{name: "too many from a group", req: LineRequest{ItemID: 1, VariantID: 10, AddonIDs: []int64{1000, 1001}}, wantErr: `from "Crust"`},
		{
			name:   "over the topping limit",
			mutate: func(i *Item) { i.AddonGroups[1].MaxSelect = 1 },
			req:    LineRequest{ItemID: 1, VariantID: 10, AddonIDs: []int64{1000, 1010, 1011}}, wantErr: `from "Toppings"`,
		},
		{name: "sold-out add-on", req: LineRequest{ItemID: 1, VariantID: 10, AddonIDs: []int64{1000, 1012}}, wantErr: "Jalapeno is sold out"},
		{name: "add-on from another item", req: LineRequest{ItemID: 1, VariantID: 10, AddonIDs: []int64{1000, 5555}}, wantErr: "invalid add-on"},
		{name: "same add-on twice", req: LineRequest{ItemID: 1, VariantID: 10, AddonIDs: []int64{1000, 1010, 1010}}, wantErr: "more than once"},
		{name: "item sold out", mutate: func(i *Item) { i.IsAvailable = false }, req: LineRequest{ItemID: 1, VariantID: 10, AddonIDs: []int64{1000}}, wantErr: "Pizza is sold out"},
		{
			name: "plain item uses base price",
			mutate: func(i *Item) {
				i.PricePaise, i.Variants, i.AddonGroups = 4500, nil, nil
			},
			req: LineRequest{ItemID: 1}, want: 4500,
		},
		{
			name: "variant sent for item without variants",
			mutate: func(i *Item) {
				i.PricePaise, i.Variants, i.AddonGroups = 4500, nil, nil
			},
			req: LineRequest{ItemID: 1, VariantID: 10}, wantErr: "has no variants",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := pizza()
			if tc.mutate != nil {
				tc.mutate(item)
			}
			got, err := priceLine(item, tc.req)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.UnitPricePaise != tc.want {
				t.Errorf("unit price = %d, want %d", got.UnitPricePaise, tc.want)
			}
		})
	}
}

func TestPriceLineSnapshotsNames(t *testing.T) {
	got, err := priceLine(pizza(), LineRequest{ItemID: 1, VariantID: 11, AddonIDs: []int64{1001, 1010}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ItemName != "Pizza" || got.VariantName != "Large" || len(got.Addons) != 2 || got.Addons[0].Name != "Cheese burst" {
		t.Errorf("snapshot missing names: %+v", got)
	}
}
