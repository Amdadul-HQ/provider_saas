package config

import "testing"

func TestFromEnv(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL": "postgres://x",
		"JWT_SECRET":   "change-me-32-bytes-minimum-secret-key",
		"VAULT_KEY":    "0000000000000000000000000000000000000000000000000000000000000000",
		"CORS_ORIGINS": "http://a, http://b",
		"APP_ENV":      "development",
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
		{"unset env means production", map[string]string{"APP_ENV": ""}, true},
		{"staging checks secrets", map[string]string{"APP_ENV": "staging"}, true},
		{"bad proxy cidr", map[string]string{"TRUSTED_PROXY_CIDRS": "10.0.0.0/33"}, true},
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

func TestNonDevRejectsDevDefaults(t *testing.T) {
	good := map[string]string{
		"APP_ENV":           "production",
		"DATABASE_URL":      "postgres://x",
		"JWT_SECRET":        "a-very-long-production-secret-value-123",
		"VAULT_KEY":         "1111111111111111111111111111111111111111111111111111111111111111",
		"META_VERIFY_TOKEN": "s3cret-verify",
	}
	cases := map[string]map[string]string{
		"dev vault key":         {"VAULT_KEY": DevVaultKey},
		"change-me verify":      {"META_VERIFY_TOKEN": "change-me"},
		"app id without secret": {"META_APP_ID": "123"},
	}
	if c, err := FromEnv(func(k string) string { return good[k] }); err != nil || c.IsDev() || c.MigrateDatabaseURL != "postgres://x" {
		t.Fatalf("good prod config rejected: %v %+v", err, c)
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			env := map[string]string{}
			for k, v := range good {
				env[k] = v
			}
			for k, v := range mod {
				env[k] = v
			}
			if _, err := FromEnv(func(k string) string { return env[k] }); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
