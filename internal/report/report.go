package report

import (
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

type Counts struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Blocked int `json:"blocked"`
	Skipped int `json:"skipped"`
}

type Phase struct {
	Phase          string   `json:"phase"`
	Target         string   `json:"target"`
	Command        string   `json:"command"`
	StartTime      string   `json:"start_time"`
	EndTime        string   `json:"end_time"`
	DurationS      float64  `json:"duration_s"`
	Status         string   `json:"status"`
	ExitCode       *int     `json:"exit_code"`
	Required       bool     `json:"required"`
	Log            string   `json:"log,omitempty"`
	Rerun          string   `json:"rerun,omitempty"`
	Diagnostic     string   `json:"diagnostic,omitempty"`
	LifecycleError string   `json:"lifecycle_error,omitempty"`
	Failure        *Failure `json:"failure,omitempty"`
}

type Failure struct {
	Phase    string `json:"phase"`
	File     string `json:"file,omitempty"`
	TestName string `json:"test_name,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message,omitempty"`
	Stack    string `json:"stack,omitempty"`
}

type Summary struct {
	SchemaVersion  int       `json:"schema_version"`
	RunID          string    `json:"run_id"`
	Status         string    `json:"status"`
	FullRegression bool      `json:"full_regression"`
	ExitCode       int       `json:"exit_code"`
	Target         string    `json:"target"`
	StartedAt      string    `json:"started_at,omitempty"`
	FinishedAt     string    `json:"finished_at,omitempty"`
	DurationS      float64   `json:"duration_s,omitempty"`
	Counts         Counts    `json:"counts"`
	Phases         []Phase   `json:"phases"`
	Failed         []Phase   `json:"failed"`
	Failures       []Failure `json:"failures"`
	LifecycleError string    `json:"lifecycle_error,omitempty"`
	Blocked        []Phase   `json:"blocked"`
	Skipped        []Phase   `json:"skipped"`
	NextAction     string    `json:"next_action"`
}

func (s Summary) MarshalJSON() ([]byte, error) {
	type alias Summary
	if s.SchemaVersion == 0 {
		s.SchemaVersion = 1
	}
	return json.Marshal(alias(s))
}

type Event struct {
	SchemaVersion int            `json:"schema_version"`
	RunID         string         `json:"run_id"`
	Sequence      int            `json:"sequence"`
	Event         string         `json:"event"`
	Timestamp     string         `json:"timestamp"`
	Payload       map[string]any `json:"-"`
}

func (e Event) MarshalJSON() ([]byte, error) {
	value := map[string]any{"schema_version": e.SchemaVersion, "run_id": e.RunID, "sequence": e.Sequence, "event": e.Event, "timestamp": e.Timestamp}
	for key, item := range e.Payload {
		value[key] = item
	}
	return json.Marshal(value)
}

type Stream struct {
	writer   io.Writer
	runID    string
	sequence int
	mutex    sync.Mutex
}

func NewStream(writer io.Writer, runID string) (*Stream, error) {
	if writer == nil || runID == "" {
		return nil, errors.New("stream writer and run ID are required")
	}
	return &Stream{writer: writer, runID: runID}, nil
}

func (s *Stream) Emit(name string, payload map[string]any) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.sequence++
	event := Event{SchemaVersion: 1, RunID: s.runID, Sequence: s.sequence, Event: name, Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Payload: payload}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = s.writer.Write(append(data, '\n'))
	return err
}
