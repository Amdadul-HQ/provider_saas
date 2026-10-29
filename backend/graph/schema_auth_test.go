package graph_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// ADR-007 role matrix guard (SECURITY H2, L-023): every Mutation field must declare
// @auth(roles: [...]). The only exceptions are the documented public fields and two
// account-level fields that work without a tenant role.
var (
	publicMutations = []string{"register", "login", "refreshToken", "logout", "acceptInvite", "placeOrder", "portalRequestRenewal"}
	accountLevel    = []string{"switchTenant", "changePassword", "logoutAll", "createEventsToken"} // bare @auth: any signed-in user
	// VIEWER is read-only: it may only call these personal / no-op mutations.
	viewerMutations = []string{"ping", "markNotificationRead", "markAllNotificationsRead"}
	// RESELLER may write only its own customers and subscriptions (plus personal ones).
	resellerMutations = []string{"ping", "markNotificationRead", "markAllNotificationsRead", "createCustomer", "updateCustomer",
		"addCustomerNote", "createSubscription", "renewSubscription"}
)

func loadSchema(t *testing.T) *ast.Schema {
	t.Helper()
	var files []string
	for _, g := range []string{"schema/*.graphql", "../internal/modules/*/*.graphql"} {
		m, err := filepath.Glob(g)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	var src []*ast.Source
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src = append(src, &ast.Source{Name: f, Input: string(b)})
	}
	s, err := gqlparser.LoadSchema(src...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func roles(f *ast.FieldDefinition) (bool, []string) {
	d := f.Directives.ForName("auth")
	if d == nil {
		return false, nil
	}
	a := d.Arguments.ForName("roles")
	if a == nil || a.Value == nil {
		return true, nil
	}
	var out []string
	for _, c := range a.Value.Children {
		out = append(out, c.Value.Raw)
	}
	return true, out
}

func TestEveryMutationDeclaresRoles(t *testing.T) {
	s := loadSchema(t)
	if s.Mutation == nil || len(s.Mutation.Fields) < 50 {
		t.Fatal("mutation type not loaded")
	}
	for _, f := range s.Mutation.Fields {
		has, rs := roles(f)
		switch {
		case slices.Contains(publicMutations, f.Name):
			if has {
				t.Errorf("%s is documented public but has @auth", f.Name)
			}
		case slices.Contains(accountLevel, f.Name):
			if !has {
				t.Errorf("%s must be @auth", f.Name)
			}
		case !has:
			t.Errorf("Mutation.%s has no @auth (add it to the role matrix, ADR-007)", f.Name)
		case len(rs) == 0:
			t.Errorf("Mutation.%s has a bare @auth; declare roles (ADR-007)", f.Name)
		default:
			if slices.Contains(rs, "VIEWER") && !slices.Contains(viewerMutations, f.Name) {
				t.Errorf("Mutation.%s allows VIEWER, which is read-only", f.Name)
			}
			if slices.Contains(rs, "RESELLER") && !slices.Contains(resellerMutations, f.Name) {
				t.Errorf("Mutation.%s allows RESELLER outside its own customers/subscriptions", f.Name)
			}
		}
	}
}

func TestSensitiveQueriesAreRestricted(t *testing.T) {
	s := loadSchema(t)
	must := map[string][]string{
		"auditLogs":      {"OWNER", "ADMIN"},
		"invoices":       {"OWNER", "ADMIN", "STAFF", "VIEWER"},
		"payments":       {"OWNER", "ADMIN", "STAFF", "VIEWER"},
		"conversations":  {"OWNER", "ADMIN", "STAFF", "VIEWER"},
		"masterAccounts": {"OWNER", "ADMIN", "STAFF", "VIEWER"},
		"dashboardStats": {"OWNER", "ADMIN", "STAFF", "VIEWER"},
	}
	for name, want := range must {
		f := s.Query.Fields.ForName(name)
		if f == nil {
			t.Fatalf("Query.%s missing", name)
		}
		if _, rs := roles(f); !slices.Equal(rs, want) {
			t.Errorf("Query.%s roles = %v, want %v", name, rs, want)
		}
	}
	if _, rs := roles(s.Mutation.Fields.ForName("revealMasterAccountCredential")); !slices.Equal(rs, []string{"OWNER", "ADMIN"}) {
		t.Errorf("revealMasterAccountCredential roles = %v", rs)
	}
}
