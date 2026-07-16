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

type OurAirportsIngestHandler struct {
	store store.Store
}

func NewOurAirportsIngestHandler(s store.Store) *OurAirportsIngestHandler {
	return &OurAirportsIngestHandler{store: s}
}

type ourAirportsIngestJobResponse struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Status model.JobStatus `json:"status"`
}

type ourAirportsIngestResponse struct {
	Job ourAirportsIngestJobResponse `json:"job"`
}

func (h *OurAirportsIngestHandler) CreateCountries(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, store.OurAirportsCountries)
}

func (h *OurAirportsIngestHandler) CreateRegions(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, store.OurAirportsRegions)
}

func (h *OurAirportsIngestHandler) CreateAirports(w http.ResponseWriter, r *http.Request) {
	h.create(w, r, store.OurAirportsAirports)
}

func (h *OurAirportsIngestHandler) create(w http.ResponseWriter, r *http.Request, dataset store.OurAirportsDataset) {
	var req model.ForceIngestRequest
	if err := decodeForceIngestRequest(r.Body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{jsonErrKey: "invalid json body"})
		return
	}

	jobType, err := ourairports.JobType(dataset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "invalid dataset"})
		return
	}

	ctx := r.Context()

	active, err := h.store.ActiveIngestJob(ctx, jobType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to check active ingest jobs"})
		return
	}

	if active {
		writeJSON(w, http.StatusConflict, map[string]any{
			jsonErrKey: "ingest job already pending or running for this dataset",
			"job_type": jobType,
		})

		return
	}

	if !req.Force {
		hasData, err := h.store.HasOurAirportsData(ctx, dataset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to check existing data"})
			return
		}

		if hasData {
			writeJSON(w, http.StatusConflict, map[string]any{
				jsonErrKey: "data already exists for this dataset; set force=true to re-import",
				"dataset":  string(dataset),
			})

			return
		}
	}

	job, err := h.store.CreateOurAirportsIngestJob(ctx, jobType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to create ingest job"})
		return
	}

	writeJSON(w, http.StatusCreated, ourAirportsIngestResponse{
		Job: ourAirportsIngestJobResponse{
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
