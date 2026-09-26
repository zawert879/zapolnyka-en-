package ui

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Job — одно выполнение команды (go, assets, check…) с накопленным логом.
// Команды выполняются по одной: они ходят в en.cx одной сессией и пишут в общий лог.
type Job struct {
	ID       int       `json:"id"`
	Cmd      string    `json:"cmd"`
	Title    string    `json:"title"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished,omitempty"`
	Done     bool      `json:"done"`
	Error    string    `json:"error,omitempty"`
	lines    []string
	buf      bytes.Buffer
	mu       sync.Mutex
}

// Write — io.Writer для логгера: режет на строки.
func (j *Job) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.buf.Write(p)
	for {
		i := bytes.IndexByte(j.buf.Bytes(), '\n')
		if i < 0 {
			break
		}
		line := strings.TrimRight(string(j.buf.Next(i+1)), "\r\n")
		j.lines = append(j.lines, stripANSI(line))
	}
	return len(p), nil
}

func (j *Job) flush() {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.buf.Len() > 0 {
		j.lines = append(j.lines, stripANSI(strings.TrimRight(j.buf.String(), "\r\n")))
		j.buf.Reset()
	}
}

// Lines возвращает строки лога начиная с from и общее число строк.
func (j *Job) Lines(from int) ([]string, int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if from < 0 {
		from = 0
	}
	if from > len(j.lines) {
		from = len(j.lines)
	}
	out := make([]string, len(j.lines)-from)
	copy(out, j.lines[from:])
	return out, len(j.lines)
}

// stripANSI убирает цветовые коды lipgloss из строк для веб-лога.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == ';') {
				j++
			}
			if j < len(s) {
				j++ // буква-команда
			}
			i = j - 1
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Jobs — менеджер заданий: по одному одновременно, история последних.
type Jobs struct {
	mu      sync.Mutex
	seq     int
	jobs    map[int]*Job
	running *Job
}

// NewJobs создаёт менеджер.
func NewJobs() *Jobs { return &Jobs{jobs: map[int]*Job{}} }

// ErrBusy — уже выполняется другое задание.
var ErrBusy = errors.New("уже выполняется другое задание — дождитесь его завершения")

// Start запускает fn в горутине. fn пишет лог в переданный writer.
func (m *Jobs) Start(cmd, title string, fn func(log *Job) error) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running != nil && !m.running.Done {
		return nil, ErrBusy
	}
	m.seq++
	j := &Job{ID: m.seq, Cmd: cmd, Title: title, Started: time.Now()}
	m.jobs[j.ID] = j
	m.running = j
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(j, "❌ panic: %v\n", r)
				j.finish(fmt.Errorf("panic: %v", r))
			}
		}()
		err := fn(j)
		j.finish(err)
	}()
	return j, nil
}

func (j *Job) finish(err error) {
	j.flush()
	j.mu.Lock()
	defer j.mu.Unlock()
	if err != nil {
		j.Error = err.Error()
		j.lines = append(j.lines, "❌ "+err.Error())
	} else {
		j.lines = append(j.lines, "✔ готово")
	}
	j.Done = true
	j.Finished = time.Now()
}

// Get возвращает задание по id.
func (m *Jobs) Get(id int) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	return j, ok
}

// Running — текущее незавершённое задание, если есть.
func (m *Jobs) Running() *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running != nil && !m.running.Done {
		return m.running
	}
	return nil
}

// Last — последнее задание (для строки статуса).
func (m *Jobs) Last() *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}
