package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

const testJobType model.JobType = "test_job"

type testHandler struct {
	jobType model.JobType
	result  json.RawMessage
	err     error
}

func (h testHandler) Type() model.JobType { return h.jobType }

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

	processor := mustNewProcessor(t, st, testHandler{jobType: testJobType, result: result})
	job := &model.Job{ID: testJobID, Type: testJobType, Status: model.JobStatusRunning}

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

	processor := mustNewProcessor(t, st, testHandler{jobType: testJobType, err: handlerErr})
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

func TestProcessorProcessFailJobIgnoresCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var failCtxErr error

	var failedMsg string

	st := &storetest.Stub{
		FailJobFn: func(failCtx context.Context, _, errMsg string) error {
			failCtxErr = failCtx.Err()
			failedMsg = errMsg

			return nil
		},
	}

	processor := mustNewProcessor(t, st, testHandler{jobType: testJobType, err: context.Canceled})
	job := &model.Job{ID: testJobID, Type: testJobType, Status: model.JobStatusRunning}

	if err := processor.Process(ctx, job); err == nil {
		t.Fatal("Process() expected error")
	}

	if failCtxErr != nil {
		t.Errorf("FailJob ctx.Err() = %v, want nil", failCtxErr)
	}

	if failedMsg != context.Canceled.Error() {
		t.Errorf("FailJob msg = %q, want %q", failedMsg, context.Canceled.Error())
	}
}

func TestProcessorProcessShutdownCauseMessage(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(ErrInterruptedByShutdown)

	var failedMsg string

	st := &storetest.Stub{
		FailJobFn: func(failCtx context.Context, _, errMsg string) error {
			if err := failCtx.Err(); err != nil {
				t.Errorf("FailJob ctx already done: %v", err)
			}

			failedMsg = errMsg

			return nil
		},
	}

	processor := mustNewProcessor(t, st, testHandler{jobType: testJobType, err: context.Canceled})
	job := &model.Job{ID: "job-3", Type: testJobType, Status: model.JobStatusRunning}

	if err := processor.Process(ctx, job); err == nil {
		t.Fatal("Process() expected error")
	}

	if failedMsg != ErrInterruptedByShutdown.Error() {
		t.Errorf("FailJob msg = %q, want %q", failedMsg, ErrInterruptedByShutdown.Error())
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

	processor := mustNewProcessor(t, st)
	job := &model.Job{ID: "job-unknown", Type: model.JobType("unknown_type"), Status: model.JobStatusRunning}

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

func TestProcessorPersistsShortJobError(t *testing.T) {
	ctx := context.Background()
	handlerErr := fmt.Errorf("load flights: %w", errors.New("copy flight_performance: secret driver detail"))

	var failedMsg string

	st := &storetest.Stub{
		FailJobFn: func(_ context.Context, _, errMsg string) error {
			failedMsg = errMsg
			return nil
		},
	}

	processor := mustNewProcessor(t, st, testHandler{jobType: testJobType, err: handlerErr})
	job := &model.Job{ID: testJobID, Type: testJobType, Status: model.JobStatusRunning}

	if err := processor.Process(ctx, job); err == nil {
		t.Fatal("Process() expected error")
	}

	if failedMsg != "load flights" {
		t.Errorf("FailJob msg = %q, want %q", failedMsg, "load flights")
	}
}

func TestNewProcessorDuplicateType(t *testing.T) {
	st := &storetest.Stub{}

	_, err := NewProcessor(st, testHandler{jobType: testJobType}, testHandler{jobType: testJobType})
	if err == nil {
		t.Fatal("NewProcessor() expected duplicate type error")
	}
}
