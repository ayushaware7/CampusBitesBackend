package menu

import (
	"strings"
	"testing"
)

func TestValidateItem(t *testing.T) {
	ok := func() ItemInput { return ItemInput{CategoryID: 1, Name: "  Burger ", PricePaise: 5000} }

	tests := []struct {
		name    string
		mutate  func(*ItemInput)
		wantErr string
	}{
		{name: "valid", mutate: func(*ItemInput) {}},
		{name: "no category", mutate: func(i *ItemInput) { i.CategoryID = 0 }, wantErr: "categoryId"},
		{name: "blank name", mutate: func(i *ItemInput) { i.Name = "   " }, wantErr: "name is required"},
		{name: "zero price without variants", mutate: func(i *ItemInput) { i.PricePaise = 0 }, wantErr: "greater than 0"},
		{name: "zero base price is fine with variants", mutate: func(i *ItemInput) {
			i.PricePaise = 0
			i.Variants = []VariantInput{{Name: "S", PricePaise: 100}}
		}},
		{name: "duplicate variant names", mutate: func(i *ItemInput) {
			i.Variants = []VariantInput{{Name: "Small", PricePaise: 100}, {Name: "small", PricePaise: 200}}
		}, wantErr: "twice"},
		{name: "variant needs a price", mutate: func(i *ItemInput) {
			i.Variants = []VariantInput{{Name: "Small"}}
		}, wantErr: "greater than 0"},
		{name: "bad image url", mutate: func(i *ItemInput) { i.ImageURL = "javascript:alert(1)" }, wantErr: "imageUrl"},
		{name: "path image url", mutate: func(i *ItemInput) { i.ImageURL = "/images/burger.jpg" }},
		{name: "duplicate group ids", mutate: func(i *ItemInput) { i.AddonGroupIDs = []int64{3, 3} }, wantErr: "unique"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := ok()
			tc.mutate(&in)
			err := validateItem(&in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if in.Name != strings.TrimSpace(in.Name) {
					t.Errorf("name not trimmed: %q", in.Name)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateGroup(t *testing.T) {
	two := []AddonInput{{Name: "A"}, {Name: "B"}}
	tests := []struct {
		name    string
		in      AddonGroupInput
		wantErr string
	}{
		{name: "valid optional group", in: AddonGroupInput{Name: "Toppings", MinSelect: 0, MaxSelect: 2, Addons: two}},
		{name: "valid required single choice", in: AddonGroupInput{Name: "Crust", MinSelect: 1, MaxSelect: 1, Addons: two}},
		{name: "max below 1", in: AddonGroupInput{Name: "X", MaxSelect: 0, Addons: two}, wantErr: "maxSelect"},
		{name: "min above max", in: AddonGroupInput{Name: "X", MinSelect: 3, MaxSelect: 2, Addons: two}, wantErr: "maxSelect"},
		{name: "min cannot be met", in: AddonGroupInput{Name: "X", MinSelect: 3, MaxSelect: 3, Addons: two}, wantErr: "more than the 2"},
		{name: "negative add-on price", in: AddonGroupInput{Name: "X", MaxSelect: 1, Addons: []AddonInput{{Name: "A", PricePaise: -1}}}, wantErr: "negative"},
		{name: "duplicate add-on names", in: AddonGroupInput{Name: "X", MaxSelect: 1, Addons: []AddonInput{{Name: "A"}, {Name: "a"}}}, wantErr: "twice"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGroup(&tc.in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
