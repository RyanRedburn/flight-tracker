package handlers

// ErrorResponse is the common JSON error body returned by the API.
type ErrorResponse struct {
	Error string `json:"error"`
}

// StatusResponse is a simple status payload used by health endpoints.
type StatusResponse struct {
	Status string `json:"status"`
}
