package handlers

const (
	errInvalidJSONBody            = "invalid json body"
	errRouteOutlookNotFound       = "route outlook not found"
	errFailedItineraryOutlook     = "failed to compute itinerary outlook"
	errFailedCheckActiveIngest    = "failed to check active ingest jobs"
	errFailedCreateIngestJob      = "failed to create ingest job"
	errFailedCheckExistingFlight  = "failed to check existing flight data"
	errFailedCheckExistingWeather = "failed to check existing weather data"
	errActiveIngestMonths         = "ingest jobs already pending or running for one or more requested months"
	errActiveReferenceIngest      = "ingest job already pending or running for this dataset"
	errActiveRebuildJob           = "rebuild job already pending"
	errFailedCheckActiveRebuild   = "failed to check active rebuild jobs"
	errFailedCreateRebuildJob     = "failed to create rebuild job"
	errFailedCreateAPIKey         = "failed to create API key"
	errFailedListAPIKeys          = "failed to list API keys"
	errFailedRevokeAPIKey         = "failed to revoke API key"
	errAPIKeyNotFound             = "key not found"
	errAPIKeyIDRequired           = "id is required"
)
