package handlers

const (
	errInvalidJSONBody            = "invalid json body"
	errFailedCheckActiveIngest    = "failed to check active ingest jobs"
	errFailedCreateIngestJob      = "failed to create ingest job"
	errFailedCheckExistingFlight  = "failed to check existing flight data"
	errFailedCheckExistingWeather = "failed to check existing weather data"
	errActiveIngestMonths         = "ingest jobs already pending or running for one or more requested months"
	errActiveReferenceIngest      = "ingest job already pending or running for this dataset"
)
