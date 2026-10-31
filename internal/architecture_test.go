package internal

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The modular-monolith rule: a feature module (auth, canteen, menu, order, payment) may import
// only the shared platform packages, never another module. Modules meet in cmd/server through
// small interfaces. This is what keeps each one splittable into its own service later.
func TestModulesDoNotImportEachOther(t *testing.T) {
	const prefix = "github.com/ayushaware7/CampusBitesBackend/internal/"
	modules := []string{"auth", "canteen", "menu", "order", "payment"}

	for _, mod := range modules {
		err := filepath.Walk(mod, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				p, _ := strconv.Unquote(imp.Path.Value)
				if !strings.HasPrefix(p, prefix) {
					continue
				}
				rest := strings.TrimPrefix(p, prefix)
				if strings.HasPrefix(rest, "platform/") {
					continue
				}
				t.Errorf("%s imports %q: modules may only import internal/platform/*; wire them together in cmd/server", path, rest)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
