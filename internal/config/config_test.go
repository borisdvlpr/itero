package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	setMinimalEnv(t)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() returned unexpected error: %v", err)
	}

	if got, want := cfg.Server.Addr(), "0.0.0.0:8000"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}

	if got, want := cfg.Server.ShutdownTimeout, 30*time.Second; got != want {
		t.Errorf("ShutdownTimeout = %v, want %v", got, want)
	}

	if !cfg.DB.RunMigrations {
		t.Error("RunMigrations = false, want true by default")
	}
}

func TestLoadConfigRequiresPassword(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("PG_PASSWORD", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() succeeded without PG_PASSWORD, want error")
	}

	if !strings.Contains(err.Error(), "PG_PASSWORD") {
		t.Errorf("error %q does not mention PG_PASSWORD", err)
	}
}

func TestLoadConfigReportsEveryProblemAtOnce(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("PG_PASSWORD", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "thirty")
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig() succeeded with three invalid values, want error")
	}

	for _, want := range []string{"PG_PASSWORD", "SHUTDOWN_TIMEOUT", "LOG_LEVEL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("aggregated error does not mention %s: %v", want, err)
		}
	}
}

func TestLoadConfigRejectsUnknownLogLevel(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("LOG_LEVEL", "warning") // valid-looking, but not a slog level

	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig() accepted LOG_LEVEL=warning, want error")
	}
}

func TestLoadConfigParsesLogLevel(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() returned unexpected error: %v", err)
	}

	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
}

func TestLoadConfigRejectsUnknownSSLMode(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("PG_SSLMODE", "off")

	if _, err := LoadConfig(); err == nil {
		t.Fatal("LoadConfig() accepted PG_SSLMODE=off, want error")
	}
}

func TestDSNEscapesCredentials(t *testing.T) {
	db := DBConfig{
		User:     "itero",
		Password: "p@ss word/1",
		Host:     "db.internal",
		Port:     "5432",
		Database: "itero",
		SSLMode:  "require",
	}

	got := db.DSN()

	if strings.Contains(got, "p@ss word/1") {
		t.Errorf("DSN() left the password unescaped: %s", got)
	}

	if !strings.Contains(got, "sslmode=require") {
		t.Errorf("DSN() = %s, want it to carry sslmode=require", got)
	}
}

func TestDBConfigLogValueRedactsPassword(t *testing.T) {
	db := DBConfig{
		User:     "itero",
		Password: "s3cret",
		Host:     "db",
		Port:     "5432",
	}

	rendered := db.LogValue().String()

	if strings.Contains(rendered, "s3cret") {
		t.Errorf("LogValue() leaked the password: %s", rendered)
	}
}

// setMinimalEnv sets everything required for a successful load. Individual
// tests override single keys to isolate the behaviour under test.
func setMinimalEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"ADDRESS", "PORT", "LOG_LEVEL", "READ_HEADER_TIMEOUT", "READ_TIMEOUT",
		"WRITE_TIMEOUT", "IDLE_TIMEOUT", "SHUTDOWN_TIMEOUT", "REQUEST_TIMEOUT",
		"MAX_HEADER_BYTES", "MAX_REQUEST_BYTES", "TRUST_PROXY_HEADERS",
		"PG_USER", "PG_HOST", "PG_PORT", "PG_DATABASE", "PG_SSLMODE",
		"PG_MAX_CONNS", "PG_MIN_CONNS", "PG_MAX_CONN_LIFETIME",
		"PG_MAX_CONN_IDLE_TIME", "PG_HEALTH_CHECK_PERIOD", "PG_CONNECT_TIMEOUT",
		"RUN_MIGRATIONS",
	} {
		t.Setenv(key, "")
	}

	t.Setenv("PG_PASSWORD", "s3cret")
}
