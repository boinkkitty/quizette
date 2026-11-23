package internal_test

import (
	"os/exec"
	"strings"
	"testing"
)

// TestDependencyRule enforces the hexagonal dependency rule:
// domain <- app <- adapters <- cmd.
func TestDependencyRule(t *testing.T) {
	t.Parallel()
	const mod = "github.com/boinkkitty/quizette/backend/"
	rules := []struct {
		pkg        string
		forbidden  []string
		stdlibOnly bool
	}{
		{pkg: "./domain/...", stdlibOnly: true},
		{pkg: "./app", forbidden: []string{mod + "internal/adapters", "github.com/jackc/pgx", "github.com/go-chi", "google.golang.org/genai", "golang.org/x/oauth2", "github.com/ledongthuc"}},
	}
	for _, r := range rules {
		out, err := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", r.pkg).Output()
		if err != nil {
			t.Fatalf("go list %s: %v", r.pkg, err)
		}
		for _, dep := range strings.Fields(string(out)) {
			if r.stdlibOnly && !strings.HasPrefix(dep, mod+"internal/domain") {
				t.Errorf("%s imports non-stdlib package %s", r.pkg, dep)
			}
			for _, f := range r.forbidden {
				if strings.HasPrefix(dep, f) {
					t.Errorf("%s depends on forbidden %s", r.pkg, dep)
				}
			}
		}
	}
}
