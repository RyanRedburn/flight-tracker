package store

import (
	"strings"
	"testing"
)

func TestQueryAdvisoryXactLock(t *testing.T) {
	if !strings.Contains(QueryAdvisoryXactLock, "pg_advisory_xact_lock") || !strings.Contains(QueryAdvisoryXactLock, "hashtext") {
		t.Fatalf("QueryAdvisoryXactLock = %q, want pg_advisory_xact_lock(hashtext(...))", QueryAdvisoryXactLock)
	}
}
