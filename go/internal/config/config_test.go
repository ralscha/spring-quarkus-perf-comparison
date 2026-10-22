package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEnvironmentOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.yml")
	contents, err := os.ReadFile("../../app.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APP_CONFIG", path)
	t.Setenv("APP_SERVER_PORT", "9090")
	t.Setenv("APP_DATABASE_MAXCONNS", "7")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != 9090 || cfg.Database.MaxConns != 7 {
		t.Fatalf("environment overrides ignored: port=%d maxConns=%d", cfg.Server.Port, cfg.Database.MaxConns)
	}
}

func TestConnectionStringEscapesCredentials(t *testing.T) {
	cfg := DatabaseConfig{Host: "::1", Port: 5432, Name: "fruits", User: "test user", Password: "password ' @ /&=", SSLMode: "disable"}
	parsed, err := pgxpool.ParseConfig(cfg.ConnString())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ConnConfig.User != cfg.User || parsed.ConnConfig.Password != cfg.Password || parsed.ConnConfig.Host != cfg.Host {
		t.Fatal("connection string changed credentials or IPv6 host")
	}
}
