package operator

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/bts"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

func newTestBTSIngestHandler(t *testing.T, store *mem.Store) *BTSIngestHandler {
	t.Helper()

	opener := func(context.Context, int, int) (string, func(), error) {
		return minimalRepoCSV(t), func() {}, nil
	}

	svc := bts.NewService(store, nil).WithCSVOpener(opener)

	return NewBTSIngestHandler(store, svc)
}

func minimalRepoCSV(t *testing.T) string {
	t.Helper()

	srcPath := filepath.Join("..", "..", "test-data",
		"On_Time_Marketing_Carrier_On_Time_Performance_Beginning_January_2018_2026_4",
		"On_Time_Marketing_Carrier_On_Time_Performance_(Beginning_January_2018)_2026_4.csv",
	)

	abs, err := filepath.Abs(srcPath)
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}

	src, err := os.Open(abs)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer src.Close()

	reader := csv.NewReader(src)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		t.Fatalf("Read header: %v", err)
	}

	row, err := reader.Read()
	if err != nil {
		t.Fatalf("Read row: %v", err)
	}

	path := filepath.Join(t.TempDir(), "minimal.csv")

	out, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	writer := csv.NewWriter(out)
	if err := writer.Write(header); err != nil {
		t.Fatalf("Write header: %v", err)
	}

	if err := writer.Write(row); err != nil {
		t.Fatalf("Write row: %v", err)
	}

	writer.Flush()

	if err := writer.Error(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if err := out.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	return path
}
