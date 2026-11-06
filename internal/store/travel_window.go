package store

const (
	// TravelWindowRebuildLockKey is held for the whole truncate+insert.
	// TravelWindowRebuildJobLockKey serializes queueing only, so a POST does
	// not block until that rebuild commits.
	TravelWindowRebuildLockKey    = "rebuild_route_travel_windows"
	TravelWindowRebuildJobLockKey = "queue_rebuild_route_travel_windows"
)

type RouteTravelWindowsFilter struct {
	Origin  string
	Dest    string
	Carrier string
}
