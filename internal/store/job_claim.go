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

func jobClaimRank(jobType model.JobType) int {
	for _, rebuildType := range rebuildClaimJobTypes {
		if jobType == rebuildType {
			return 1
		}
	}

	return 0
}

func jobClaimLess(a, b model.Job) bool {
	if rankA, rankB := jobClaimRank(a.Type), jobClaimRank(b.Type); rankA != rankB {
		return rankA < rankB
	}

	return a.CreatedAt.Before(b.CreatedAt)
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
