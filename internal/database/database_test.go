//go:build cgo

package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/config"
)

func TestNewStorePostgresNotImplemented(t *testing.T) {
	_, err := NewStore(context.Background(), config.Config{DatabaseDriver: "postgres"})
	if !errors.Is(err, errPostgresNotImplemented) {
		t.Fatalf("NewStore() error = %v, want errPostgresNotImplemented", err)
	}
}

func TestNewStoreUnsupportedDriver(t *testing.T) {
	_, err := NewStore(context.Background(), config.Config{DatabaseDriver: "mysql"})
	if err == nil {
		t.Fatal("NewStore() expected error for unsupported driver")
	}
}

func TestNewStoreSQLite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	cfg := config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    "file:" + filepath.ToSlash(dbPath),
		MigrationsPath: filepath.Join("..", "..", "..", "migrations", "sqlite"),
	}

	store, err := NewStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}
