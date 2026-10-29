package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordHashing(t *testing.T) {
	h, err := HashPassword("Demo1234!")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		pw   string
		want bool
	}{{"Demo1234!", true}, {"demo1234!", false}, {"", false}}
	for _, tt := range tests {
		ok, err := VerifyPassword(tt.pw, h)
		if err != nil || ok != tt.want {
			t.Errorf("VerifyPassword(%q) = %v,%v want %v", tt.pw, ok, err, tt.want)
		}
	}
	if _, err := VerifyPassword("x", "$bcrypt$nope"); err == nil {
		t.Error("expected invalid hash error")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	iss := NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Minute)
	p := Principal{UserID: uuid.New(), TenantID: uuid.New(), Role: RoleOwner}
	tok, _, err := iss.Issue(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := iss.Parse(tok)
	if err != nil || got != p {
		t.Fatalf("Parse = %+v, %v", got, err)
	}
	other := NewTokenIssuer("another-secret-another-secret-123456", time.Minute)
	if _, err := other.Parse(tok); err == nil {
		t.Fatal("token verified with wrong secret")
	}
	expired := NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Minute)
	expired.now = func() time.Time { return time.Now().Add(-time.Hour) }
	old, _, _ := expired.Issue(p)
	if _, err := iss.Parse(old); err == nil {
		t.Fatal("expired token accepted")
	}
}

func TestMiddleware(t *testing.T) {
	iss := NewTokenIssuer("0123456789abcdef0123456789abcdef", time.Minute)
	p := Principal{UserID: uuid.New(), TenantID: uuid.New(), Role: RoleStaff}
	tok, _, _ := iss.Issue(p)
	var got Principal
	var ok bool
	h := Middleware(iss)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = FromContext(r.Context())
	}))
	req := httptest.NewRequest("POST", "/graphql", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !ok || got != p {
		t.Fatalf("principal = %+v %v", got, ok)
	}
	req = httptest.NewRequest("POST", "/graphql", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if ok {
		t.Fatal("garbage token produced principal")
	}
}

func TestRefreshToken(t *testing.T) {
	a, ha, err := NewRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _, _ := NewRefreshToken()
	if a == b || HashToken(a) != ha || len(ha) != 64 {
		t.Fatal("refresh tokens not random / hash mismatch")
	}
	if !(Principal{Role: "admin"}).HasRole("OWNER", "ADMIN") || (Principal{Role: "staff"}).HasRole("OWNER") {
		t.Fatal("HasRole")
	}
}
