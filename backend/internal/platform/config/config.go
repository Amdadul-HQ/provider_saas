// Package config loads and validates process configuration from the environment
// (names match .env.example). A local .env file is loaded in development without
// overriding variables that are already set.
package config

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv      string
	HTTPAddr    string
	DatabaseURL string
	// MigrateDatabaseURL connects as the table owner for migrations (default DatabaseURL).
	// The API itself must connect as the non-owner, NOBYPASSRLS role psaas_app (ADR-002).
	MigrateDatabaseURL string
	DBMaxConns         int32
	RedisURL           string
	JWTSecret          string
	JWTAccessTTL       time.Duration
	JWTRefreshTTL      time.Duration
	// VaultKey/VaultKeyVersion are the ACTIVE key (new secrets are sealed with it).
	VaultKey        []byte
	VaultKeyVersion int
	// VaultKeys holds every key version that can decrypt (VAULT_KEYS + VAULT_KEY).
	VaultKeys map[int][]byte
	PublicAppURL       string
	PublicAPIURL       string
	UploadDir          string
	CORSOrigins        []string
	MetaAppID          string
	MetaAppSecret      string
	MetaVerifyToken    string
	MetaGraphVersion   string
	SMSWebhookSecret   string
	RateLimitRPS       float64
	RateLimitBurst     int
	ComplexityLimit    int
	// TrustedProxyCIDRs: X-Forwarded-For / X-Real-IP are honoured only from these peers.
	TrustedProxyCIDRs []string
}

// IsDev is true only for an explicit APP_ENV=development or test (SECURITY M5: an unset
// APP_ENV means production, with masked errors, no playground and strict secret checks).
func (c Config) IsDev() bool {
	return c.AppEnv == "development" || c.AppEnv == "test"
}
func (c Config) IsProd() bool { return c.AppEnv == "production" }

// Load reads the environment (after loading .env files if present) and validates.
func Load() (Config, error) {
	LoadDotEnv(".env", "../.env")
	return FromEnv(os.Getenv)
}

// FromEnv builds a Config from a lookup function (testable).
func FromEnv(get func(string) string) (Config, error) {
	var errs []error
	str := func(k, def string) string {
		if v := strings.TrimSpace(get(k)); v != "" {
			return v
		}
		return def
	}
	dur := func(k, def string) time.Duration {
		d, err := time.ParseDuration(str(k, def))
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", k, err))
		}
		return d
	}
	num := func(k string, def int) int {
		v := str(k, strconv.Itoa(def))
		n, err := strconv.Atoi(v)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", k, err))
		}
		return n
	}
	c := Config{
		AppEnv:           strings.ToLower(str("APP_ENV", "production")),
		HTTPAddr:         str("HTTP_ADDR", ":8080"),
		DatabaseURL:      str("DATABASE_URL", ""),
		DBMaxConns:       int32(num("DB_MAX_CONNS", 20)),
		RedisURL:         str("REDIS_URL", ""),
		JWTSecret:        str("JWT_SECRET", ""),
		JWTAccessTTL:     dur("JWT_ACCESS_TTL", "15m"),
		JWTRefreshTTL:    dur("JWT_REFRESH_TTL", "720h"),
		VaultKeyVersion:  num("VAULT_KEY_VERSION", 1),
		PublicAppURL:     str("PUBLIC_APP_URL", "http://localhost:3000"),
		PublicAPIURL:     strings.TrimRight(str("PUBLIC_API_URL", "http://localhost:8080"), "/"),
		UploadDir:        str("UPLOAD_DIR", "./uploads"),
		MetaAppID:        str("META_APP_ID", ""),
		MetaAppSecret:    str("META_APP_SECRET", ""),
		MetaVerifyToken:  str("META_VERIFY_TOKEN", ""),
		MetaGraphVersion: str("META_GRAPH_VERSION", "v21.0"),
		SMSWebhookSecret: str("SMS_WEBHOOK_SECRET", ""),
		RateLimitBurst:   num("RATE_LIMIT_BURST", 60),
		ComplexityLimit:  num("GRAPHQL_COMPLEXITY_LIMIT", 1000),
	}
	rps, err := strconv.ParseFloat(str("RATE_LIMIT_RPS", "20"), 64)
	if err != nil {
		errs = append(errs, fmt.Errorf("RATE_LIMIT_RPS: %w", err))
	}
	c.RateLimitRPS = rps
	for _, o := range strings.Split(str("CORS_ORIGINS", "http://localhost:3000"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, o)
		}
	}

	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	c.MigrateDatabaseURL = str("MIGRATE_DATABASE_URL", c.DatabaseURL)
	for _, p := range strings.Split(str("TRUSTED_PROXY_CIDRS", ""), ",") {
		if p = strings.TrimSpace(p); p != "" {
			if _, _, err := net.ParseCIDR(p); err != nil {
				errs = append(errs, fmt.Errorf("TRUSTED_PROXY_CIDRS: %w", err))
				continue
			}
			c.TrustedProxyCIDRs = append(c.TrustedProxyCIDRs, p)
		}
	}
	if len(c.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET must be at least 32 bytes"))
	}
	c.VaultKeys, c.VaultKeyVersion, errs = vaultKeys(str, c.VaultKeyVersion, errs)
	c.VaultKey = c.VaultKeys[c.VaultKeyVersion]
	if !c.IsDev() { // every environment except development/test (staging included)
		if strings.Contains(c.JWTSecret, "change-me") {
			errs = append(errs, fmt.Errorf("JWT_SECRET must be changed (APP_ENV=%s)", c.AppEnv))
		}
		for ver, key := range c.VaultKeys {
			if isZero(key) || hex.EncodeToString(key) == DevVaultKey {
				errs = append(errs, fmt.Errorf("vault key v%d must be a real secret, not the dev key (APP_ENV=%s)", ver, c.AppEnv))
			}
		}
		if strings.Contains(c.MetaVerifyToken, "change-me") {
			errs = append(errs, fmt.Errorf("META_VERIFY_TOKEN must be changed (APP_ENV=%s)", c.AppEnv))
		}
		if c.MetaAppID != "" && c.MetaAppSecret == "" {
			errs = append(errs, errors.New("META_APP_SECRET is required when META_APP_ID is set"))
		}
	}
	return c, errors.Join(errs...)
}

// vaultKeys parses the vault key ring (SECURITY L3):
//
//	VAULT_KEYS="v1:<64 hex>,v2:<64 hex>"   every version that may still decrypt
//	VAULT_ACTIVE_KEY_VERSION=2              the version new secrets are sealed with
//	VAULT_KEY=<64 hex> + VAULT_KEY_VERSION   single-key form (still supported; merged in)
//
// The active version defaults to VAULT_KEY_VERSION when VAULT_KEY is set, else to the
// highest version in VAULT_KEYS.
func vaultKeys(str func(k, def string) string, singleVersion int, errs []error) (map[int][]byte, int, []error) {
	keys := map[int][]byte{}
	if v := str("VAULT_KEYS", ""); v != "" {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name, hexKey, ok := strings.Cut(part, ":")
			ver, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(name), "v"))
			if !ok || err != nil || ver < 1 {
				errs = append(errs, errors.New(`VAULT_KEYS entries must look like "v1:<64 hex chars>"`))
				continue
			}
			key, err := hex.DecodeString(strings.TrimSpace(hexKey))
			if err != nil || len(key) != 32 {
				errs = append(errs, fmt.Errorf("VAULT_KEYS v%d must be 64 hex chars (32 bytes)", ver))
				continue
			}
			if _, dup := keys[ver]; dup {
				errs = append(errs, fmt.Errorf("VAULT_KEYS lists v%d twice", ver))
			}
			keys[ver] = key
		}
	}
	active := 0
	if v := str("VAULT_KEY", ""); v != "" || len(keys) == 0 {
		key, err := hex.DecodeString(v)
		if err != nil || len(key) != 32 {
			errs = append(errs, errors.New("VAULT_KEY must be 64 hex chars (32 bytes)"))
		} else {
			if singleVersion < 1 {
				errs = append(errs, errors.New("VAULT_KEY_VERSION must be >= 1"))
			}
			if old, ok := keys[singleVersion]; ok && !bytes.Equal(old, key) {
				errs = append(errs, fmt.Errorf("VAULT_KEY differs from VAULT_KEYS v%d", singleVersion))
			}
			keys[singleVersion] = key
			active = singleVersion
		}
	}
	if v := str("VAULT_ACTIVE_KEY_VERSION", ""); v != "" {
		n, err := strconv.Atoi(strings.TrimPrefix(v, "v"))
		if err != nil {
			errs = append(errs, fmt.Errorf("VAULT_ACTIVE_KEY_VERSION: %w", err))
		}
		active = n
	} else if active == 0 {
		for ver := range keys {
			active = max(active, ver)
		}
	}
	if _, ok := keys[active]; !ok && len(keys) > 0 {
		errs = append(errs, fmt.Errorf("VAULT_ACTIVE_KEY_VERSION %d is not in VAULT_KEYS", active))
	}
	return keys, active, errs
}

// DevVaultKey is the committed development VAULT_KEY (.env.example, Makefile). It is
// rejected outside development/test.
const DevVaultKey = "6465762d6f6e6c792d7661756c742d6b65792d646f2d6e6f742d7573652d2121"

func isZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

// LoadDotEnv sets variables from the first existing file; existing env wins.
// Supports KEY=VALUE, quotes, and trailing " # comments" on unquoted values.
func LoadDotEnv(paths ...string) {
	for _, p := range paths {
		f, err := os.Open(filepath.Clean(p))
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			k, v, ok := parseLine(sc.Text())
			if !ok {
				continue
			}
			if _, set := os.LookupEnv(k); !set {
				_ = os.Setenv(k, v)
			}
		}
		_ = f.Close()
		return
	}
}

func parseLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")
	k, v, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false
	}
	k, v = strings.TrimSpace(k), strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return k, v[1 : end+1], true
		}
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return k, v, k != ""
}
