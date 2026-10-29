package sqlb

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"provider_saas/internal/platform/pagination"
)

var tid = uuid.MustParse("11111111-1111-1111-1111-111111111111")

func ptr[T any](v T) *T { return &v }

func TestFiltersNumberPlaceholders(t *testing.T) {
	q := New("c.tenant_id", tid).Where("c.deleted_at IS NULL")
	Eq(q, "c.status", ptr("active"))
	Eq[string](q, "c.source", nil)
	q.WhereIf(true, "c.tags @> ?::text[]", []string{"vip"})
	q.WhereIf(false, "c.due_balance > ?", 0)
	q.Where("c.created_at BETWEEN ? AND ?", 1, 2)

	want := " WHERE c.tenant_id = $1 AND c.deleted_at IS NULL AND c.status = $2 AND c.tags @> $3::text[] AND c.created_at BETWEEN $4 AND $5"
	if got := q.WhereSQL(); got != want {
		t.Fatalf("WhereSQL\n got %s\nwant %s", got, want)
	}
	if want := []any{tid, "active", []string{"vip"}, 1, 2}; !reflect.DeepEqual(q.Args(), want) {
		t.Fatalf("Args = %v", q.Args())
	}
}

func TestWherePanicsOnCountMismatch(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	New("tenant_id", tid).Where("a = ? AND b = ?", 1)
}

func TestSearch(t *testing.T) {
	t.Run("blank term adds nothing", func(t *testing.T) {
		for _, term := range []*string{nil, ptr(""), ptr("   ")} {
			q := New("t.tenant_id", tid).Search(term, Like("t.title"))
			if q.WhereSQL() != " WHERE t.tenant_id = $1" || len(q.Args()) != 1 {
				t.Fatalf("term %v added %s", term, q.WhereSQL())
			}
		}
	})
	t.Run("fields share one pattern and one exact value", func(t *testing.T) {
		q := New("t.tenant_id", tid).Search(ptr("  50%_off "),
			Like("t.title", "t.description"),
			Exact("t.number::text"),
			Related("customers c", "c.tenant_id = t.tenant_id AND c.id = t.customer_id", Like("c.name", "c.phone")))
		want := " WHERE t.tenant_id = $1 AND (t.title ILIKE $2 OR t.description ILIKE $2 OR t.number::text = $3 OR " +
			"EXISTS (SELECT 1 FROM customers c WHERE c.tenant_id = t.tenant_id AND c.id = t.customer_id AND (c.name ILIKE $2 OR c.phone ILIKE $2)))"
		if got := q.WhereSQL(); got != want {
			t.Fatalf("WhereSQL\n got %s\nwant %s", got, want)
		}
		if want := []any{tid, `%50\%\_off%`, "50%_off"}; !reflect.DeepEqual(q.Args(), want) {
			t.Fatalf("Args = %#v", q.Args())
		}
	})
	t.Run("long terms are cut", func(t *testing.T) {
		q := New("t.tenant_id", tid).Search(ptr(strings.Repeat("অ", MaxSearchLen+20)), Exact("t.x"))
		if got := q.Args()[1].(string); len([]rune(got)) != MaxSearchLen {
			t.Fatalf("term has %d runes", len([]rune(got)))
		}
	})
}

func TestPage(t *testing.T) {
	ks := Keyset{Sort: "c.created_at", ID: "c.id", From: "customers c"}
	at := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	id := uuid.New()

	t.Run("first page", func(t *testing.T) {
		q := New("c.tenant_id", tid)
		got := q.Page(pagination.Page{First: 25}, ks)
		if want := " WHERE c.tenant_id = $1 ORDER BY c.created_at DESC, c.id DESC LIMIT $2"; got != want {
			t.Fatalf("got %s", got)
		}
		if q.Args()[1] != 26 {
			t.Fatalf("limit = %v, want first+1", q.Args()[1])
		}
	})
	t.Run("opaque cursor", func(t *testing.T) {
		q := New("c.tenant_id", tid)
		got := q.Page(pagination.Page{First: 10, After: &pagination.Cursor{CreatedAt: at, ID: id}}, ks)
		want := " WHERE c.tenant_id = $1 AND (c.created_at, c.id) < ($2, $3) ORDER BY c.created_at DESC, c.id DESC LIMIT $4"
		if got != want {
			t.Fatalf("got %s", got)
		}
		if !reflect.DeepEqual(q.Args(), []any{tid, at, id, 11}) {
			t.Fatalf("Args = %v", q.Args())
		}
	})
	t.Run("row id cursor", func(t *testing.T) {
		q := New("c.tenant_id", tid)
		got := q.Page(pagination.Page{First: 10, AfterID: &id}, ks)
		want := " WHERE c.tenant_id = $1 AND (c.created_at, c.id) < (SELECT c.created_at, c.id FROM customers c WHERE c.tenant_id = $1 AND c.id = $2)" +
			" ORDER BY c.created_at DESC, c.id DESC LIMIT $3"
		if got != want {
			t.Fatalf("got %s", got)
		}
	})
	t.Run("zero first falls back to default", func(t *testing.T) {
		q := New("c.tenant_id", tid)
		q.Page(pagination.Page{}, ks)
		if q.Args()[1] != pagination.DefaultFirst+1 {
			t.Fatalf("limit = %v", q.Args()[1])
		}
	})
}

func TestLimit(t *testing.T) {
	q := New("tenant_id", tid).Where("deleted_at IS NULL")
	if got, want := q.Limit("sort_order, name", 50), " WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY sort_order, name LIMIT $2"; got != want {
		t.Fatalf("got %s", got)
	}
}

func TestNumberTerm(t *testing.T) {
	if NumberTerm(nil) != nil || *NumberTerm(ptr(" #1024 ")) != "1024" || *NumberTerm(ptr("rahim")) != "rahim" {
		t.Fatal("NumberTerm")
	}
}
