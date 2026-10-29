// Package config loads and validates process configuration from the environment
// (names match .env.example). A local .env file is loaded in development without
// overriding variables that are already set.
package config

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv           string
	HTTPAddr         string
	DatabaseURL      string
	DBMaxConns       int32
	RedisURL         string
	JWTSecret        string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
	VaultKey         []byte
	VaultKeyVersion  int
	PublicAppURL     string
	CORSOrigins      []string
	MetaAppID        string
	MetaAppSecret    string
	MetaVerifyToken  string
	SMSWebhookSecret string
	RateLimitRPS     float64
	RateLimitBurst   int
	ComplexityLimit  int
}

func (c Config) IsDev() bool {
	return c.AppEnv == "development" || c.AppEnv == "dev" || c.AppEnv == "test"
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
		AppEnv:           str("APP_ENV", "development"),
		HTTPAddr:         str("HTTP_ADDR", ":8080"),
		DatabaseURL:      str("DATABASE_URL", ""),
		DBMaxConns:       int32(num("DB_MAX_CONNS", 20)),
		RedisURL:         str("REDIS_URL", ""),
		JWTSecret:        str("JWT_SECRET", ""),
		JWTAccessTTL:     dur("JWT_ACCESS_TTL", "15m"),
		JWTRefreshTTL:    dur("JWT_REFRESH_TTL", "720h"),
		VaultKeyVersion:  num("VAULT_KEY_VERSION", 1),
		PublicAppURL:     str("PUBLIC_APP_URL", "http://localhost:3000"),
		MetaAppID:        str("META_APP_ID", ""),
		MetaAppSecret:    str("META_APP_SECRET", ""),
		MetaVerifyToken:  str("META_VERIFY_TOKEN", ""),
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
	if len(c.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET must be at least 32 bytes"))
	}
	key, err := hex.DecodeString(str("VAULT_KEY", ""))
	if err != nil || len(key) != 32 {
		errs = append(errs, errors.New("VAULT_KEY must be 64 hex chars (32 bytes)"))
	}
	c.VaultKey = key
	if c.VaultKeyVersion < 1 {
		errs = append(errs, errors.New("VAULT_KEY_VERSION must be >= 1"))
	}
	if c.IsProd() {
		if strings.Contains(c.JWTSecret, "change-me") {
			errs = append(errs, errors.New("JWT_SECRET must be changed in production"))
		}
		if isZero(key) {
			errs = append(errs, errors.New("VAULT_KEY must not be all zeros in production"))
		}
	}
	return c, errors.Join(errs...)
}

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
