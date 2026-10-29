//go:build integration

package app_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/google/uuid"

	"provider_saas/internal/modules/customer"
	"provider_saas/internal/modules/ticket"
	"provider_saas/internal/platform/pagination"
)

// TestListPagingAndSearch runs the shared list builder (ADR-010) against Postgres: walking a
// list with either cursor form visits every row exactly once, and search fields escape LIKE
// wildcards and reach related tables.
func TestListPagingAndSearch(t *testing.T) {
	f := newTenant(t, "Paging Store")
	names := []string{"Rahim 50% off", "Karim", "Salma", "Nusrat", "Tanvir"}
	byName := map[string]customer.Customer{}
	for i, n := range names {
		c, err := testC.Customers.Create(f.ctx, f.tid, f.uid, customer.CreateInput{Name: n, Phone: strp(fmt.Sprintf("0171100000%d", i))})
		if err != nil {
			t.Fatal(err)
		}
		byName[n] = c
	}

	walk := func(t *testing.T, next func(last customer.Customer) pagination.Page) {
		t.Helper()
		seen := map[uuid.UUID]bool{}
		page := pagination.Page{First: 2}
		for range len(names) {
			items, more, total, err := testC.Customers.List(f.ctx, f.tid, customer.Filter{}, page)
			if err != nil {
				t.Fatal(err)
			}
			if total != len(names) || len(items) > 2 {
				t.Fatalf("page has %d items, total %d", len(items), total)
			}
			for _, c := range items {
				if seen[c.ID] {
					t.Fatalf("%q appeared on two pages", c.Name)
				}
				seen[c.ID] = true
			}
			if !more {
				break
			}
			page = next(items[len(items)-1])
		}
		if len(seen) != len(names) {
			t.Fatalf("walked %d of %d customers", len(seen), len(names))
		}
	}
	t.Run("opaque cursor", func(t *testing.T) {
		walk(t, func(last customer.Customer) pagination.Page {
			return pagination.Page{First: 2, After: &pagination.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}}
		})
	})
	t.Run("row id cursor", func(t *testing.T) {
		walk(t, func(last customer.Customer) pagination.Page {
			id := last.ID
			return pagination.Page{First: 2, AfterID: &id}
		})
	})
	t.Run("cursor strings from the API", func(t *testing.T) {
		walk(t, func(last customer.Customer) pagination.Page {
			raw := last.ID.String() // what arrayPageInfo sends
			p, err := pagination.New(intp(2), &raw)
			if err != nil {
				t.Fatal(err)
			}
			return p
		})
	})

	search := func(term string) []string {
		t.Helper()
		items, _, total, err := testC.Customers.List(f.ctx, f.tid, customer.Filter{Search: strp(term)}, pagination.Page{First: 10})
		if err != nil {
			t.Fatal(err)
		}
		if total != len(items) {
			t.Fatalf("search %q: count %d but %d rows", term, total, len(items))
		}
		out := make([]string, len(items))
		for i, c := range items {
			out[i] = c.Name
		}
		return out
	}
	if got := search("50%"); len(got) != 1 || got[0] != "Rahim 50% off" {
		t.Fatalf(`search "50%%" = %v`, got)
	}
	if got := search("_"); len(got) != 0 {
		t.Fatalf(`search "_" matched %v; the wildcard must be escaped`, got)
	}
	if got := search(" 01711000003 "); len(got) != 1 || got[0] != "Nusrat" {
		t.Fatalf("phone search = %v", got)
	}

	// Related (customer name) and Exact ("#number") fields on tickets.
	tk, err := testC.Tickets.Create(f.ctx, f.tid, f.uid, ticket.CreateInput{CustomerID: byName["Salma"].ID, Kind: "question", Title: "Login does not work"})
	if err != nil {
		t.Fatal(err)
	}
	for _, term := range []string{"salma", "#" + strconv.Itoa(tk.Number), "login"} {
		ts, err := testC.Tickets.List(f.ctx, f.tid, ticket.Filter{Search: strp(term)}, pagination.Page{First: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(ts) != 1 || ts[0].ID != tk.ID {
			t.Fatalf("ticket search %q found %d tickets", term, len(ts))
		}
	}
}

func intp(n int) *int { return &n }
