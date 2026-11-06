package main

import (
	"context"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/config"
)

func TestOpenStoreUnsupportedDriver(t *testing.T) {
	_, err := openStore(context.Background(), config.Config{DatabaseDriver: "sqlite"})
	if err == nil {
		t.Fatal("openStore() expected error for unsupported driver")
	}

	_, err = openStore(context.Background(), config.Config{DatabaseDriver: "mysql"})
	if err == nil {
		t.Fatal("openStore() expected error for unsupported driver")
	}
}
