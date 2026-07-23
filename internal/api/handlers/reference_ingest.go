package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/ourairports"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type ReferenceIngestHandler struct {
	store store.Store
}

func NewReferenceIngestHandler(s store.Store) *ReferenceIngestHandler {
	return &ReferenceIngestHandler{store: s}
}

// ReferenceIngestJobResponse is the queued reference-data ingest job.
type ReferenceIngestJobResponse struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Status model.JobStatus `json:"status"`
}

// ReferenceIngestResponse is returned after successfully queueing a reference-data ingest job.
type ReferenceIngestResponse struct {
	Job ReferenceIngestJobResponse `json:"job"`
}

// CreateCountries queues a countries reference data ingest job.
//
//	@Summary		Queue countries reference data ingest
//	@Description	Queues an import of countries reference data. An empty body is treated as {"force":false}. Set force=true to replace existing data.
//	@Tags			ingest,internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.ForceIngestRequest	false	"Optional force flag"
//	@Success		201		{object}	ReferenceIngestResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ReferenceIngestConflictResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/ingest/countries [post]
func (h *ReferenceIngestHandler) CreateCountries(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, store.ReferenceCountries)
}

// CreateRegions queues a regions reference data ingest job.
//
//	@Summary		Queue regions reference data ingest
//	@Description	Queues an import of regions reference data. An empty body is treated as {"force":false}. Set force=true to replace existing data.
//	@Tags			ingest,internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.ForceIngestRequest	false	"Optional force flag"
//	@Success		201		{object}	ReferenceIngestResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ReferenceIngestConflictResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/ingest/regions [post]
func (h *ReferenceIngestHandler) CreateRegions(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, store.ReferenceRegions)
}

// CreateAirports queues an airports reference data ingest job.
//
//	@Summary		Queue airports reference data ingest
//	@Description	Queues an import of airports reference data. An empty body is treated as {"force":false}. Set force=true to replace existing data.
//	@Tags			ingest,internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.ForceIngestRequest	false	"Optional force flag"
//	@Success		201		{object}	ReferenceIngestResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	ReferenceIngestConflictResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/ingest/airports [post]
func (h *ReferenceIngestHandler) CreateAirports(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, store.ReferenceAirports)
}

func (h *ReferenceIngestHandler) create(w http.ResponseWriter, r *http.Request, dataset store.ReferenceDataset) {
	var req model.ForceIngestRequest
	if err := decodeForceIngestRequest(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: errInvalidJSONBody})
		return
	}

	jobType, err := ourairports.JobType(dataset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "invalid dataset"})
		return
	}

	ctx := r.Context()

	active, err := h.store.ActiveIngestJob(ctx, jobType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCheckActiveIngest})
		return
	}

	if active {
		writeJSON(w, http.StatusConflict, ReferenceIngestConflictResponse{
			Error:   "ingest job already pending or running for this dataset",
			JobType: jobType,
		})

		return
	}

	if !req.Force {
		hasData, err := h.store.HasReferenceData(ctx, dataset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to check existing data"})
			return
		}

		if hasData {
			writeJSON(w, http.StatusConflict, ReferenceIngestConflictResponse{
				Error:   "data already exists for this dataset; set force=true to re-import",
				Dataset: string(dataset),
			})

			return
		}
	}

	job, err := h.store.CreateReferenceIngestJob(ctx, jobType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCreateIngestJob})
		return
	}

	writeJSON(w, http.StatusCreated, ReferenceIngestResponse{
		Job: ReferenceIngestJobResponse{
			ID:     job.ID,
			Type:   job.Type,
			Status: job.Status,
		},
	})
}

func decodeForceIngestRequest(body io.Reader, req *model.ForceIngestRequest) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(req); err != nil {
		if errors.Is(err, io.EOF) {
			*req = model.ForceIngestRequest{}
			return nil
		}

		return err
	}

	return nil
}
