package gql

import (
	"context"
	"strings"
	"testing"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func opCtx(t *testing.T, q string) *graphql.OperationContext {
	t.Helper()
	doc, err := parser.ParseQuery(&ast.Source{Input: q})
	if err != nil {
		t.Fatal(err)
	}
	return &graphql.OperationContext{Doc: doc, Operation: doc.Operations[0]}
}

func TestLimits(t *testing.T) {
	deep := "{" + strings.Repeat("a {", 12) + "b" + strings.Repeat("}", 12) + "}" // depth 13
	ok12 := "{" + strings.Repeat("a {", 11) + "b" + strings.Repeat("}", 11) + "}"
	cases := []struct {
		name, q string
		reject  bool
	}{
		{"single login", `mutation { login(input:{identifier:"a",password:"b"}) { accessToken } }`, false},
		{"aliased logins", `mutation { a: login(input:{identifier:"a",password:"1"}) { accessToken } b: login(input:{identifier:"a",password:"2"}) { accessToken } }`, true},
		{"login + register", `mutation { login(input:{identifier:"a",password:"1"}) { accessToken } register(input:{}) { accessToken } }`, true},
		{"login via fragment twice", `mutation { ...F ...G } fragment F on Mutation { login(input:{identifier:"a",password:"1"}) { accessToken } } fragment G on Mutation { login(input:{identifier:"b",password:"1"}) { accessToken } }`, true},
		{"six aliases", `{ a1: me { id } a2: me { id } a3: me { id } a4: me { id } a5: me { id } a6: me { id } }`, true},
		{"five aliases", `{ a1: me { id } a2: me { id } a3: me { id } a4: me { id } a5: me { id } }`, false},
		{"depth 13", deep, true},
		{"depth 12", ok12, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := DefaultLimits.MutateOperationContext(context.Background(), opCtx(t, c.q))
			if (err != nil) != c.reject {
				t.Fatalf("reject=%v err=%v", c.reject, err)
			}
		})
	}
}
