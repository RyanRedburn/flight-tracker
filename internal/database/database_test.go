package database

import (
	"context"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/config"
)

func TestNewStoreUnsupportedDriver(t *testing.T) {
	_, err := NewStore(context.Background(), config.Config{DatabaseDriver: "sqlite"})
	if err == nil {
		t.Fatal("NewStore() expected error for unsupported driver")
	}

	_, err = NewStore(context.Background(), config.Config{DatabaseDriver: "mysql"})
	if err == nil {
		t.Fatal("NewStore() expected error for unsupported driver")
	}
}
