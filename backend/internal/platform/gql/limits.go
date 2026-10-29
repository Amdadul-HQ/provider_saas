package gql

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// Limits rejects abusive operations before execution (SECURITY H6, L10):
//   - selection depth above MaxDepth;
//   - more than MaxAliases aliased fields in the whole operation;
//   - more than one "sensitive" root field per operation (login, register, ...), so one
//     HTTP request cannot carry many password guesses or public writes.
type Limits struct {
	MaxDepth   int
	MaxAliases int
	// Sensitive root Mutation/Query fields allowed at most once per operation.
	Sensitive map[string]bool
}

// DefaultLimits are the production limits.
var DefaultLimits = Limits{
	MaxDepth:   12,
	MaxAliases: 5,
	Sensitive: map[string]bool{
		"login": true, "register": true, "refreshToken": true, "acceptInvite": true, "changePassword": true,
		"placeOrder": true, "portalRequestRenewal": true, "publicOrder": true, "invite": true,
	},
}

var _ interface {
	graphql.HandlerExtension
	graphql.OperationContextMutator
} = Limits{}

func (Limits) ExtensionName() string                   { return "OperationLimits" }
func (Limits) Validate(graphql.ExecutableSchema) error { return nil }

func limitErr(msg string) *gqlerror.Error {
	return &gqlerror.Error{Message: msg, Extensions: map[string]any{"code": "OPERATION_LIMIT"}}
}

func (l Limits) MutateOperationContext(_ context.Context, rc *graphql.OperationContext) *gqlerror.Error {
	if rc == nil || rc.Operation == nil {
		return nil
	}
	w := walker{doc: rc.Doc}
	depth := w.depth(rc.Operation.SelectionSet, 1, map[string]bool{})
	if l.MaxDepth > 0 && depth > l.MaxDepth {
		return limitErr("query is too deep")
	}
	if l.MaxAliases > 0 && w.aliases > l.MaxAliases {
		return limitErr("too many aliases in one operation")
	}
	n := 0
	for _, name := range w.rootFields(rc.Operation.SelectionSet, map[string]bool{}) {
		if l.Sensitive[name] {
			n++
		}
	}
	if n > 1 {
		return limitErr("only one sign-in or public write field is allowed per request")
	}
	return nil
}

type walker struct {
	doc     *ast.QueryDocument
	aliases int
}

func (w *walker) fragment(name string) *ast.FragmentDefinition {
	if w.doc == nil {
		return nil
	}
	return w.doc.Fragments.ForName(name)
}

// depth returns the maximum field depth and counts aliases (each fragment once per path).
func (w *walker) depth(set ast.SelectionSet, level int, seen map[string]bool) int {
	maxD := level - 1
	for _, sel := range set {
		switch s := sel.(type) {
		case *ast.Field:
			if s.Alias != "" && s.Alias != s.Name {
				w.aliases++
			}
			d := level
			if len(s.SelectionSet) > 0 {
				d = w.depth(s.SelectionSet, level+1, seen)
			}
			maxD = max(maxD, d)
		case *ast.InlineFragment:
			maxD = max(maxD, w.depth(s.SelectionSet, level, seen))
		case *ast.FragmentSpread:
			if seen[s.Name] {
				continue // cycles are rejected by validation; never loop here
			}
			if f := w.fragment(s.Name); f != nil {
				seen[s.Name] = true
				maxD = max(maxD, w.depth(f.SelectionSet, level, seen))
				delete(seen, s.Name)
			}
		}
	}
	return maxD
}

// rootFields lists the root field names (through fragments), with repetitions.
func (w *walker) rootFields(set ast.SelectionSet, seen map[string]bool) []string {
	var out []string
	for _, sel := range set {
		switch s := sel.(type) {
		case *ast.Field:
			out = append(out, s.Name)
		case *ast.InlineFragment:
			out = append(out, w.rootFields(s.SelectionSet, seen)...)
		case *ast.FragmentSpread:
			if f := w.fragment(s.Name); f != nil && !seen[s.Name] {
				seen[s.Name] = true
				out = append(out, w.rootFields(f.SelectionSet, seen)...)
			}
		}
	}
	return out
}
