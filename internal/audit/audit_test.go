package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	l, err := New(logPath)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// Test Log with specific time
	now := time.Now().UTC().Truncate(time.Second)
	evt1 := Event{
		Time:   now,
		Action: "create-bucket",
		Method: "PUT",
		Path:   "/test-bucket",
		Status: 200,
	}
	l.Log(evt1)

	// Test Log with zero time (should default to Now)
	evt2 := Event{
		Action: "put-object",
		Method: "PUT",
		Path:   "/test-bucket/obj",
		Status: 200,
	}
	l.Log(evt2)

	if err := l.Close(); err != nil {
		t.Fatalf("failed to close logger: %v", err)
	}

	// Verify file content
	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var events []Event
	for scanner.Scan() {
		var evt Event
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			t.Fatalf("failed to unmarshal log line: %v", err)
		}
		events = append(events, evt)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if !events[0].Time.Equal(evt1.Time) {
		t.Errorf("expected time %v, got %v", evt1.Time, events[0].Time)
	}
	if events[0].Action != evt1.Action {
		t.Errorf("expected action %s, got %s", evt1.Action, events[0].Action)
	}

	if events[1].Time.IsZero() {
		t.Error("expected non-zero time for second event")
	}
	if events[1].Action != evt2.Action {
		t.Errorf("expected action %s, got %s", evt2.Action, events[1].Action)
	}
}

func TestLogger_NilAndClose(t *testing.T) {
	var l *Logger
	// Should not panic
	l.Log(Event{Action: "test"})
	if err := l.Close(); err != nil {
		t.Errorf("Close on nil logger should return nil, got %v", err)
	}

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit2.log")
	l2, _ := New(logPath)
	l2.Close()
	// Should not panic or error significantly if used after close
	l2.Log(Event{Action: "test after close"})
	_ = l2.Close()
}

func TestNew_Error(t *testing.T) {
	// Try to create logger in a non-existent directory
	_, err := New("/non-existent/path/audit.log")
	if err == nil {
		t.Error("expected error when creating logger in invalid path")
	}
}
