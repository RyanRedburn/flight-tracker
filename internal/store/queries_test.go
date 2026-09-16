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

func TestQueryJobLeaseSQL(t *testing.T) {
	if !strings.Contains(QueryClaimNextPendingJobUpdate, "lease_expires_at") {
		t.Fatalf("QueryClaimNextPendingJobUpdate = %q, want lease_expires_at", QueryClaimNextPendingJobUpdate)
	}

	if !strings.Contains(QueryHeartbeatJob, "lease_expires_at") {
		t.Fatalf("QueryHeartbeatJob = %q, want lease_expires_at", QueryHeartbeatJob)
	}

	if !strings.Contains(QueryResetStaleRunningJobs, "lease_expires_at") {
		t.Fatalf("QueryResetStaleRunningJobs = %q, want lease_expires_at", QueryResetStaleRunningJobs)
	}

	if strings.Contains(QueryResetStaleRunningJobs, "started_at <") {
		t.Fatalf("QueryResetStaleRunningJobs = %q, should expire by lease not started_at", QueryResetStaleRunningJobs)
	}

	if !strings.Contains(QueryFailJob, "lease_expires_at = NULL") {
		t.Fatalf("QueryFailJob = %q, want lease_expires_at = NULL", QueryFailJob)
	}

	if !strings.Contains(QueryCompleteJob, "lease_expires_at = NULL") {
		t.Fatalf("QueryCompleteJob = %q, want lease_expires_at = NULL", QueryCompleteJob)
	}
}
