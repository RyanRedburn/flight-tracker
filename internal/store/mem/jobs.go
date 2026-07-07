package mem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/uuid"
)

func (s *Store) CreateBTSIngestJob(ctx context.Context, year, month int) (*model.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	job := &model.Job{
		ID:        uuid.NewString(),
		Type:      model.JobTypeImportBTSOnTime,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.jobs[job.ID] = cloneJob(job)
	s.btsIngest[job.ID] = model.BTSIngestJob{
		JobID: job.ID,
		Year:  year,
		Month: month,
	}

	return cloneJob(job), nil
}

func (s *Store) GetBTSIngestJob(ctx context.Context, jobID string) (*model.BTSIngestJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	detail, ok := s.btsIngest[jobID]
	if !ok {
		return nil, fmt.Errorf("bts ingest job %q: %w", jobID, store.ErrNotFound)
	}

	copy := detail
	return &copy, nil
}

func (s *Store) ClaimNextPendingJob(ctx context.Context) (*model.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var oldest *model.Job

	for _, job := range s.jobs {
		if job.Status != model.JobStatusPending {
			continue
		}

		if oldest == nil || job.CreatedAt.Before(oldest.CreatedAt) {
			oldest = job
		}
	}

	if oldest == nil {
		return nil, store.ErrNotFound
	}

	now := time.Now().UTC()
	claimed := cloneJob(oldest)
	claimed.Status = model.JobStatusRunning
	claimed.StartedAt = &now
	claimed.UpdatedAt = now
	s.jobs[claimed.ID] = claimed

	return cloneJob(claimed), nil
}

func (s *Store) CompleteJob(ctx context.Context, id string, result json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job %q: %w", id, store.ErrNotFound)
	}

	if job.Status != model.JobStatusRunning {
		return store.ErrJobStatusConflict
	}

	now := time.Now().UTC()
	updated := cloneJob(job)
	updated.Status = model.JobStatusCompleted
	updated.Result = append(json.RawMessage(nil), result...)
	updated.Error = ""
	updated.EndedAt = &now
	updated.UpdatedAt = now
	s.jobs[id] = updated

	return nil
}

func (s *Store) FailJob(ctx context.Context, id, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job %q: %w", id, store.ErrNotFound)
	}

	if job.Status != model.JobStatusRunning {
		return store.ErrJobStatusConflict
	}

	now := time.Now().UTC()
	updated := cloneJob(job)
	updated.Status = model.JobStatusFailed
	updated.Error = errMsg
	updated.EndedAt = &now
	updated.UpdatedAt = now
	s.jobs[id] = updated

	return nil
}

func (s *Store) ResetStaleRunningJobs(ctx context.Context, olderThan time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var reset int64

	for id, job := range s.jobs {
		if job.Status != model.JobStatusRunning || job.StartedAt == nil {
			continue
		}

		if !job.StartedAt.Before(olderThan) {
			continue
		}

		updated := cloneJob(job)
		updated.Status = model.JobStatusPending
		updated.StartedAt = nil
		updated.UpdatedAt = time.Now().UTC()
		s.jobs[id] = updated
		reset++
	}

	return reset, nil
}

func (s *Store) ActiveBTSIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	requested := make(map[model.YearMonth]struct{}, len(months))
	for _, ym := range months {
		requested[ym] = struct{}{}
	}

	activeSet := make(map[model.YearMonth]struct{})
	var active []model.YearMonth

	for jobID, detail := range s.btsIngest {
		ym := model.YearMonth{Year: detail.Year, Month: detail.Month}
		if _, ok := requested[ym]; !ok {
			continue
		}

		job, ok := s.jobs[jobID]
		if !ok {
			continue
		}

		if job.Status != model.JobStatusPending && job.Status != model.JobStatusRunning {
			continue
		}

		if _, seen := activeSet[ym]; seen {
			continue
		}

		activeSet[ym] = struct{}{}
		active = append(active, ym)
	}

	return active, nil
}

func (s *Store) MonthsWithOnTimeFlightData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var withData []model.YearMonth

	for _, ym := range months {
		for _, flight := range s.flights {
			year, month, ok := store.FlightYearMonthFromDate(flight.FlightDate)
			if !ok {
				continue
			}

			if year == ym.Year && month == ym.Month {
				withData = append(withData, ym)
				break
			}
		}
	}

	return withData, nil
}

func (s *Store) ReplaceOnTimeFlightsByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error {
	if len(columns) == 0 {
		return errors.New("columns required")
	}

	colIndex := make(map[string]int, len(columns))
	for i, col := range columns {
		colIndex[col] = i
	}

	flightDateIdx, ok := colIndex["FlightDate"]
	if !ok {
		return errors.New("FlightDate column required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	remaining := make([]*model.OnTimeFlight, 0, len(s.flights))
	for _, flight := range s.flights {
		fy, fm, ok := store.FlightYearMonthFromDate(flight.FlightDate)
		if ok && fy == year && fm == month {
			continue
		}

		remaining = append(remaining, cloneOnTimeFlight(flight))
	}

	for _, row := range rows {
		if len(row) != len(columns) {
			return fmt.Errorf("row width %d does not match columns %d", len(row), len(columns))
		}

		flight := &model.OnTimeFlight{FlightDate: row[flightDateIdx]}
		if idx, ok := colIndex["Origin"]; ok {
			flight.Origin = row[idx]
		}
		if idx, ok := colIndex["Dest"]; ok {
			flight.Dest = row[idx]
		}
		if idx, ok := colIndex["IATA_Code_Marketing_Airline"]; ok {
			flight.IATA_Code_Marketing_Airline = row[idx]
		}
		if idx, ok := colIndex["Flight_Number_Marketing_Airline"]; ok {
			flight.Flight_Number_Marketing_Airline = row[idx]
		}
		if idx, ok := colIndex["IATA_Code_Operating_Airline"]; ok {
			flight.IATA_Code_Operating_Airline = row[idx]
		}
		if idx, ok := colIndex["Flight_Number_Operating_Airline"]; ok {
			flight.Flight_Number_Operating_Airline = row[idx]
		}
		if idx, ok := colIndex["CRSDepTime"]; ok {
			flight.CRSDepTime = row[idx]
		}
		if idx, ok := colIndex["DepTime"]; ok {
			flight.DepTime = row[idx]
		}
		if idx, ok := colIndex["DepDelay"]; ok {
			flight.DepDelay = row[idx]
		}
		if idx, ok := colIndex["CRSArrTime"]; ok {
			flight.CRSArrTime = row[idx]
		}
		if idx, ok := colIndex["ArrTime"]; ok {
			flight.ArrTime = row[idx]
		}
		if idx, ok := colIndex["ArrDelay"]; ok {
			flight.ArrDelay = row[idx]
		}
		if idx, ok := colIndex["Cancelled"]; ok {
			flight.Cancelled = row[idx]
		}
		if idx, ok := colIndex["Diverted"]; ok {
			flight.Diverted = row[idx]
		}
		if idx, ok := colIndex["Distance"]; ok {
			flight.Distance = row[idx]
		}

		remaining = append(remaining, flight)
	}

	s.flights = remaining
	return nil
}
