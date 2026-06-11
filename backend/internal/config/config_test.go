package config

import "testing"

// ShouldAutoMigrate defaults to OFF (unset) — the server does not migrate on
// boot unless `auto_migrate: true` is set explicitly (turnkey deploys).
func TestShouldAutoMigrate_Default(t *testing.T) {
	t.Parallel()
	tru, fls := true, false
	cases := []struct {
		name string
		val  *bool
		want bool
	}{
		{"unset defaults off", nil, false},
		{"explicit true", &tru, true},
		{"explicit false", &fls, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Database{AutoMigrate: c.val}.ShouldAutoMigrate()
			if got != c.want {
				t.Fatalf("ShouldAutoMigrate() = %v, want %v", got, c.want)
			}
		})
	}
}
