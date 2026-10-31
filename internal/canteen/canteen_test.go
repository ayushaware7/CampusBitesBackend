package canteen

import "testing"

func TestBrandingValidate(t *testing.T) {
	long := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = 'a'
		}
		return string(b)
	}
	cases := []struct {
		name string
		in   BrandingInput
		ok   bool
	}{
		{"name only", BrandingInput{Name: "Spice Route"}, true},
		{"all fields", BrandingInput{Name: "Spice Route", Tagline: "Fresh", LogoURL: "https://x.com/a.png", PrimaryColor: "#0066cc"}, true},
		{"empty name", BrandingInput{Name: "  "}, false},
		{"name too long", BrandingInput{Name: long(101)}, false},
		{"tagline too long", BrandingInput{Name: "A", Tagline: long(201)}, false},
		{"bad color", BrandingInput{Name: "A", PrimaryColor: "blue"}, false},
		{"short color", BrandingInput{Name: "A", PrimaryColor: "#fff"}, false},
		{"javascript logo", BrandingInput{Name: "A", LogoURL: "javascript:alert(1)"}, false},
		{"relative logo", BrandingInput{Name: "A", LogoURL: "/a.png"}, false},
		{"logo too long", BrandingInput{Name: "A", LogoURL: "https://x.com/" + long(500)}, false},
	}
	for _, c := range cases {
		if err := c.in.validate(); (err == nil) != c.ok {
			t.Errorf("%s: got err=%v, want ok=%v", c.name, err, c.ok)
		}
	}
}

func TestBrandingValidateTrims(t *testing.T) {
	in := BrandingInput{Name: "  Spice Route ", PrimaryColor: " #0066CC "}
	if err := in.validate(); err != nil || in.Name != "Spice Route" || in.PrimaryColor != "#0066CC" {
		t.Errorf("got %+v, err=%v", in, err)
	}
}
