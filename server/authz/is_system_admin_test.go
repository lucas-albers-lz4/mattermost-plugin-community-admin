package authz

import (
	"strings"
	"testing"
)

// TestIsSystemAdminTokenBoundary pins the exact-token semantics of
// IsSystemAdmin. Regression guard for the substring-match bug (#60):
// any role token merely containing "system_admin" must NOT grant admin.
func TestIsSystemAdminTokenBoundary(t *testing.T) {
	tests := []struct {
		name  string
		roles string
		want  bool
	}{
		{name: "exact token", roles: "system_admin", want: true},
		{name: "single token list", roles: "system_user system_admin", want: true},
		{name: "admin first", roles: "system_admin system_user", want: true},
		{name: "multiple roles with admin", roles: "system_user system_admin team_admin", want: true},
		{name: "tab separated", roles: "system_user\tsystem_admin", want: true},
		{name: "non-admin", roles: "system_user", want: false},
		{name: "prefix collision", roles: "not_system_admin", want: false},
		{name: "suffix collision", roles: "system_admin_audit", want: false},
		{name: "infix collision", roles: "my_system_admin_backup", want: false},
		{name: "prefix collision in list", roles: "system_user not_system_admin", want: false},
		{name: "suffix collision in list", roles: "system_user system_admin_audit", want: false},
		{name: "empty", roles: "", want: false},
		{name: "admin token only", roles: " system_admin ", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSystemAdmin(tt.roles); got != tt.want {
				t.Errorf("IsSystemAdmin(%q) = %v, want %v", tt.roles, got, tt.want)
			}
		})
	}
}

// FuzzIsSystemAdmin checks the invariant: IsSystemAdmin(roles) is true
// iff the exact token "system_admin" appears in the whitespace-split
// role set. Any divergence from token-set membership is a bug.
func FuzzIsSystemAdmin(f *testing.F) {
	// Seeds: legit admin, non-admin, and every false-positive shape from
	// the substring-match bug (#60).
	for _, seed := range []string{
		"system_user system_admin",
		"system_admin system_user",
		"system_user",
		"not_system_admin",
		"system_admin_audit",
		"my_system_admin_backup",
		"system_user not_system_admin",
		"",
		"system_admin",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, roles string) {
		want := false
		for _, tok := range strings.Fields(roles) {
			if tok == "system_admin" {
				want = true
				break
			}
		}
		if got := IsSystemAdmin(roles); got != want {
			t.Errorf("IsSystemAdmin(%q) = %v, want %v (token membership)", roles, got, want)
		}
	})
}
