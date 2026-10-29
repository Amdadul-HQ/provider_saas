package config

import "testing"

func TestFromEnv(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL": "postgres://x",
		"JWT_SECRET":   "change-me-32-bytes-minimum-secret-key",
		"VAULT_KEY":    "0000000000000000000000000000000000000000000000000000000000000000",
		"CORS_ORIGINS": "http://a, http://b",
	}
	tests := []struct {
		name    string
		mod     map[string]string
		wantErr bool
	}{
		{"valid dev", nil, false},
		{"missing db", map[string]string{"DATABASE_URL": ""}, true},
		{"short jwt", map[string]string{"JWT_SECRET": "short"}, true},
		{"bad vault", map[string]string{"VAULT_KEY": "abcd"}, true},
		{"prod default secrets", map[string]string{"APP_ENV": "production"}, true},
		{"bad ttl", map[string]string{"JWT_ACCESS_TTL": "soon"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{}
			for k, v := range base {
				env[k] = v
			}
			for k, v := range tt.mod {
				env[k] = v
			}
			c, err := FromEnv(func(k string) string { return env[k] })
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && (len(c.CORSOrigins) != 2 || !c.IsDev()) {
				t.Fatalf("unexpected config %+v", c)
			}
		})
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct{ in, k, v string }{
		{"A=1", "A", "1"},
		{"VAULT_KEY=00ff   # 32-byte hex", "VAULT_KEY", "00ff"},
		{`B="x # y"`, "B", "x # y"},
		{"export C=3", "C", "3"},
	}
	for _, tt := range tests {
		k, v, ok := parseLine(tt.in)
		if !ok || k != tt.k || v != tt.v {
			t.Errorf("parseLine(%q) = %q,%q,%v", tt.in, k, v, ok)
		}
	}
	if _, _, ok := parseLine("# comment"); ok {
		t.Error("comment parsed")
	}
}
