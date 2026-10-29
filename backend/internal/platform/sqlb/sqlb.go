// Package sqlb builds the WHERE / ORDER BY / LIMIT tail of tenant list queries: numbered bind
// parameters, declared search fields and keyset pagination (ADR-010). Every list repository
// uses it, so no repository counts $n by hand.
//
//	q := sqlb.New("c.tenant_id", tenantID).Where("c.deleted_at IS NULL")
//	sqlb.Eq(q, "c.status", f.Status)
//	q.Search(f.Search, sqlb.Like("c.name", "c.phone"))
//	rows, err := conn.Query(ctx, "SELECT "+cols+" FROM customers c"+q.Page(page, keyset), q.Args()...)
package sqlb

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"provider_saas/internal/platform/db"
	"provider_saas/internal/platform/pagination"
)

// MaxSearchLen caps a search term (in runes); longer input is cut, never rejected.
const MaxSearchLen = 100

// Query collects AND-ed conditions and their bind values. The zero value is not usable;
// start with New.
type Query struct {
	tenantCol string
	args      []any
	conds     []string
}

// New starts a query scoped to one tenant: `tenantCol = $1` is always the first condition
// (ADR-002), next to RLS.
func New(tenantCol string, tenantID uuid.UUID) *Query {
	q := &Query{tenantCol: tenantCol}
	q.conds = append(q.conds, tenantCol+" = "+q.Arg(tenantID))
	return q
}

// Arg binds v and returns its placeholder ("$n"). Use it when one value appears several
// times in a condition, or in SQL that needs a literal "?" (jsonb operators).
func (q *Query) Arg(v any) string {
	q.args = append(q.args, v)
	return "$" + strconv.Itoa(len(q.args))
}

// Where adds a condition; each "?" in cond binds the next value. A placeholder/value count
// mismatch is a programming error and panics.
func (q *Query) Where(cond string, vals ...any) *Query {
	q.conds = append(q.conds, q.bind(cond, vals))
	return q
}

// WhereIf adds the condition only when ok. Values are evaluated by the caller, so pass
// pointers (pgx binds *T) rather than dereferencing a possibly nil field.
func (q *Query) WhereIf(ok bool, cond string, vals ...any) *Query {
	if ok {
		q.Where(cond, vals...)
	}
	return q
}

// Eq adds `col = *v` when v is set (the usual optional filter field).
func Eq[T any](q *Query, col string, v *T) *Query {
	if v != nil {
		q.Where(col+" = ?", *v)
	}
	return q
}

func (q *Query) bind(cond string, vals []any) string {
	if n := strings.Count(cond, "?"); n != len(vals) {
		panic(fmt.Sprintf("sqlb: %q has %d placeholders but %d values", cond, n, len(vals)))
	}
	if len(vals) == 0 {
		return cond
	}
	var b strings.Builder
	i := 0
	for _, r := range cond {
		if r == '?' {
			b.WriteString(q.Arg(vals[i]))
			i++
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Search adds one condition that ORs fields together when term is not blank. The term is
// trimmed and cut to MaxSearchLen; LIKE wildcards in it are escaped (SECURITY I1).
func (q *Query) Search(term *string, fields ...Field) *Query {
	if term == nil || len(fields) == 0 {
		return q
	}
	t := strings.TrimSpace(*term)
	if t == "" {
		return q
	}
	if r := []rune(t); len(r) > MaxSearchLen {
		t = strings.TrimSpace(string(r[:MaxSearchLen]))
	}
	m := &matcher{q: q, term: t}
	q.conds = append(q.conds, m.or(fields))
	return q
}

// NumberTerm drops a leading "#" from a search term, so "#1024" finds ticket/order 1024
// through an Exact("x.number::text") field.
func NumberTerm(term *string) *string {
	if term == nil {
		return nil
	}
	t := strings.TrimPrefix(strings.TrimSpace(*term), "#")
	return &t
}

// Field is one searchable thing: columns matched by pattern or equality, or a related table.
type Field struct{ sql func(m *matcher) string }

// Like matches each column against %term% (case-insensitive).
func Like(cols ...string) Field {
	return Field{func(m *matcher) string { return m.each(cols, "ILIKE", m.like()) }}
}

// Exact matches each column for equality with the trimmed term, e.g. "o.number::text".
func Exact(cols ...string) Field {
	return Field{func(m *matcher) string { return m.each(cols, "=", m.exact()) }}
}

// Related matches rows of another table: EXISTS (SELECT 1 FROM from WHERE on AND (fields)).
// on must scope the related table to the outer row and its tenant.
func Related(from, on string, fields ...Field) Field {
	return Field{func(m *matcher) string {
		return "EXISTS (SELECT 1 FROM " + from + " WHERE " + on + " AND " + m.or(fields) + ")"
	}}
}

// matcher binds the pattern and the exact term once, however many fields use them.
type matcher struct {
	q                 *Query
	term              string
	likeArg, exactArg string
}

func (m *matcher) like() string {
	if m.likeArg == "" {
		m.likeArg = m.q.Arg(db.Contains(m.term))
	}
	return m.likeArg
}

func (m *matcher) exact() string {
	if m.exactArg == "" {
		m.exactArg = m.q.Arg(m.term)
	}
	return m.exactArg
}

func (m *matcher) each(cols []string, op, arg string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = c + " " + op + " " + arg
	}
	return strings.Join(parts, " OR ")
}

func (m *matcher) or(fields []Field) string {
	parts := make([]string, len(fields))
	for i, f := range fields {
		parts[i] = f.sql(m)
	}
	return "(" + strings.Join(parts, " OR ") + ")"
}

// WhereSQL returns " WHERE c1 AND c2 ..." (never empty: the tenant condition is always there).
func (q *Query) WhereSQL() string { return " WHERE " + strings.Join(q.conds, " AND ") }

// Args returns the bind values in placeholder order.
func (q *Query) Args() []any { return q.args }

// Keyset describes a newest-first ordering for cursor pagination.
type Keyset struct {
	Sort string // ordering expression, a timestamp: "c.created_at", "COALESCE(c.last_message_at, c.created_at)"
	ID   string // unique tie-breaker: "c.id"
	From string // table and alias used to resolve a row-id cursor: "customers c"
}

// Page adds the keyset condition for page (opaque cursor or last row id) and returns the
// whole tail: WHERE ... ORDER BY sort DESC, id DESC LIMIT first+1. Call it last; the extra
// row tells pagination.Trim whether there is a next page.
func (q *Query) Page(p pagination.Page, k Keyset) string {
	switch {
	case p.After != nil:
		q.Where("("+k.Sort+", "+k.ID+") < (?, ?)", p.After.CreatedAt, p.After.ID)
	case p.AfterID != nil:
		// the inner alias shadows the outer one, so Sort/ID/tenantCol resolve against the cursor row
		q.Where("("+k.Sort+", "+k.ID+") < (SELECT "+k.Sort+", "+k.ID+" FROM "+k.From+
			" WHERE "+q.tenantCol+" = $1 AND "+k.ID+" = ?)", *p.AfterID)
	}
	first := p.First
	if first <= 0 {
		first = pagination.DefaultFirst
	}
	return q.WhereSQL() + " ORDER BY " + k.Sort + " DESC, " + k.ID + " DESC LIMIT " + q.Arg(first+1)
}

// Limit returns the tail for an unpaged list: WHERE ... ORDER BY order LIMIT n.
func (q *Query) Limit(order string, n int) string {
	return q.WhereSQL() + " ORDER BY " + order + " LIMIT " + q.Arg(n)
}
