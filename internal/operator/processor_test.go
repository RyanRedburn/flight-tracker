package operator

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

const testJobType = "test_job"

type testHandler struct {
	jobType string
	result  json.RawMessage
	err     error
}

func (h testHandler) Type() string { return h.jobType }

func (h testHandler) Process(ctx context.Context, job *model.Job) (json.RawMessage, error) {
	if h.err != nil {
		return nil, h.err
	}

	return h.result, nil
}

func TestProcessorProcessSuccess(t *testing.T) {
	ctx := context.Background()
	result := json.RawMessage(`{"ok":true}`)

	var completedID string
	var completedResult json.RawMessage

	st := &storetest.Stub{
		CompleteJobFn: func(_ context.Context, id string, res json.RawMessage) error {
			completedID = id
			completedResult = append(json.RawMessage(nil), res...)

			return nil
		},
	}

	processor := NewProcessor(st, testHandler{jobType: testJobType, result: result})
	job := &model.Job{ID: "job-1", Type: testJobType, Status: model.JobStatusRunning}

	if err := processor.Process(ctx, job); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if completedID != job.ID {
		t.Errorf("CompleteJob id = %q, want %q", completedID, job.ID)
	}

	if string(completedResult) != string(result) {
		t.Errorf("CompleteJob result = %s, want %s", completedResult, result)
	}
}

func TestProcessorProcessHandlerFailure(t *testing.T) {
	ctx := context.Background()
	handlerErr := errors.New("handler failed")

	var failedID, failedMsg string

	st := &storetest.Stub{
		FailJobFn: func(_ context.Context, id, errMsg string) error {
			failedID = id
			failedMsg = errMsg

			return nil
		},
	}

	processor := NewProcessor(st, testHandler{jobType: testJobType, err: handlerErr})
	job := &model.Job{ID: "job-2", Type: testJobType, Status: model.JobStatusRunning}

	if err := processor.Process(ctx, job); err == nil {
		t.Fatal("Process() expected error")
	}

	if failedID != job.ID {
		t.Errorf("FailJob id = %q, want %q", failedID, job.ID)
	}

	if failedMsg != handlerErr.Error() {
		t.Errorf("FailJob msg = %q, want %q", failedMsg, handlerErr.Error())
	}
}

func TestProcessorProcessUnknownJobType(t *testing.T) {
	ctx := context.Background()

	var failedID, failedMsg string

	st := &storetest.Stub{
		FailJobFn: func(_ context.Context, id, errMsg string) error {
			failedID = id
			failedMsg = errMsg

			return nil
		},
	}

	processor := NewProcessor(st)
	job := &model.Job{ID: "job-unknown", Type: "unknown_type", Status: model.JobStatusRunning}

	if err := processor.Process(ctx, job); err == nil {
		t.Fatal("Process() expected error")
	}

	if failedID != job.ID {
		t.Errorf("FailJob id = %q, want %q", failedID, job.ID)
	}

	if failedMsg == "" {
		t.Error("expected FailJob error message")
	}
}
