package audit

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Event struct {
	Time      time.Time `json:"time"`
	Action    string    `json:"action"`
	Method    string    `json:"method,omitempty"`
	Path      string    `json:"path,omitempty"`
	Status    int       `json:"status,omitempty"`
	Principal string    `json:"principal,omitempty"`
	Remote    string    `json:"remote,omitempty"`
	Message   string    `json:"message,omitempty"`
}

type Logger struct {
	mu sync.Mutex
	f  *os.File
}

func New(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &Logger{f: f}, nil
}

func (l *Logger) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	return l.f.Close()
}

func (l *Logger) Log(evt Event) {
	if l == nil || l.f == nil {
		return
	}
	if evt.Time.IsZero() {
		evt.Time = time.Now().UTC()
	}
	b, err := json.Marshal(evt)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.f.Write(append(b, '\n'))
}
