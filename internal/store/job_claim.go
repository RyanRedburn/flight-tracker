package store

import (
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const rebuildTypesPlaceholder = "/*rebuild_types*/"

// rebuildClaimJobTypes are claimed after every other job type. Add new rollup
// rebuild types here so claim order and the pending-job SELECT stay together.
var rebuildClaimJobTypes = []model.JobType{
	model.JobTypeRebuildRouteTravelWindows,
	model.JobTypeRebuildRouteWeatherStats,
}

// ClaimNextPendingJobQuery is QueryClaimNextPendingJobSelect with the rebuild
// type list filled in. $1 is pending and $2 is running. A pending rebuild whose
// type already has a running row is left for a later claim.
func ClaimNextPendingJobQuery() string {
	quoted := make([]string, len(rebuildClaimJobTypes))
	for i, jobType := range rebuildClaimJobTypes {
		quoted[i] = "'" + string(jobType) + "'"
	}

	return strings.ReplaceAll(QueryClaimNextPendingJobSelect, rebuildTypesPlaceholder, strings.Join(quoted, ", "))
}
