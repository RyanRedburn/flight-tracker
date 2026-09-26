package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestProcessorPersistsWrappedJobError(t *testing.T) {
	ctx := context.Background()
	handlerErr := fmt.Errorf("download iem csv: %w", errors.New("unexpected status 502 Bad Gateway"))

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

	if failedMsg != handlerErr.Error() {
		t.Errorf("FailJob msg = %q, want %q", failedMsg, handlerErr.Error())
	}
}

func TestShortJobError(t *testing.T) {
	longHead := `download iem csv: Get "https://mesonet.example/asos?`
	longTail := "connection reset by peer"
	longMsg := longHead + strings.Repeat("x", jobErrorMaxLen) + ": " + longTail

	// Place a multibyte rune across the head cut so truncation stays valid UTF-8.
	headCut := (jobErrorMaxLen - len(jobErrorOmit)) / 2
	utf8Msg := strings.Repeat("a", headCut-1) + "é" + strings.Repeat("b", jobErrorMaxLen) + longTail

	tests := []struct {
		name       string
		err        error
		want       string
		wantPrefix string
		wantSuffix string
	}{
		{
			name: "plain",
			err:  errors.New("handler failed"),
			want: "handler failed",
		},
		{
			name: "wrapped http status",
			err:  fmt.Errorf("download iem csv: %w", errors.New("unexpected status 502 Bad Gateway")),
			want: "download iem csv: unexpected status 502 Bad Gateway",
		},
		{
			name: "wrapped service status",
			err:  errors.New("download iem csv: status 503 Service Unavailable"),
			want: "download iem csv: status 503 Service Unavailable",
		},
		{
			name: "multi segment chain",
			err: fmt.Errorf("download iem csv: %w", fmt.Errorf("Get %q: %w",
				"https://mesonet.example/asos", errors.New("connection reset by peer"))),
			want: `download iem csv: Get "https://mesonet.example/asos": connection reset by peer`,
		},
		{
			name:       "long message keeps head and tail",
			err:        errors.New(longMsg),
			wantPrefix: longHead,
			wantSuffix: longTail,
		},
		{
			name:       "long message stays valid utf8",
			err:        errors.New(utf8Msg),
			wantPrefix: strings.Repeat("a", headCut-1),
			wantSuffix: longTail,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortJobError(tt.err)

			if len(got) > jobErrorMaxLen {
				t.Errorf("len = %d, want <= %d", len(got), jobErrorMaxLen)
			}

			if !utf8.ValidString(got) {
				t.Fatalf("invalid UTF-8: %q", got)
			}

			if tt.want != "" && got != tt.want {
				t.Errorf("shortJobError() = %q, want %q", got, tt.want)
			}

			if tt.wantPrefix != "" && !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("shortJobError() = %q, want prefix %q", got, tt.wantPrefix)
			}

			if tt.wantSuffix != "" && !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("shortJobError() = %q, want suffix %q", got, tt.wantSuffix)
			}

			if tt.wantPrefix != "" && !strings.Contains(got, jobErrorOmit) {
				t.Errorf("shortJobError() = %q, want %q", got, jobErrorOmit)
			}
		})
	}
}

func TestNewProcessorDuplicateType(t *testing.T) {
	st := &storetest.Stub{}

	_, err := NewProcessor(st, testHandler{jobType: testJobType}, testHandler{jobType: testJobType})
	if err == nil {
		t.Fatal("NewProcessor() expected duplicate type error")
	}
}
