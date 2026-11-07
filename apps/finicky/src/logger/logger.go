package logger

import (
	"bytes"
	"finicky/util"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// windowWriter implements io.Writer to send logs to the window
type windowWriter struct{}

var windowHandlerMu sync.RWMutex
var windowHandler func(string)

func SetWindowHandler(handler func(string)) {
	windowHandlerMu.Lock()
	windowHandler = handler
	windowHandlerMu.Unlock()
}

func (w *windowWriter) Write(p []byte) (n int, err error) {
	// Remove trailing newline if present
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	windowHandlerMu.RLock()
	handler := windowHandler
	windowHandlerMu.RUnlock()
	if handler != nil {
		handler(msg)
	}
	return len(p), nil
}

// createHandler creates a slog handler with the given writer
func createHandler(writer io.Writer) slog.Handler {
	return slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format time as ISO string with microseconds
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   slog.TimeKey,
					Value: slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05.000000")),
				}
			}
			return a
		},
	})
}

// Setup initializes a persistent sink; changing file logging never leaves an
// old slog handler holding a closed file descriptor.
func Setup() {
	logs.reset()
	slog.SetDefault(slog.New(createHandler(io.MultiWriter(logs, os.Stdout, &windowWriter{}))))
}

// SetupFile updates the existing sink. Repeated enables reuse its open file;
// disabling synchronizes with in-flight writes and closes it before returning.
func SetupFile(shouldLog bool) error {
	if !shouldLog {
		return logs.setFile(false, "")
	}
	homeDir, err := util.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}
	return logs.setFile(true, filepath.Join(homeDir, "Library", "Logs", "Finicky Plus"))
}

func Close() {
	slog.Info("Application closed!")
	_ = logs.setFile(false, "")
}

const maxMemoryLog = 1024 * 1024

var logs = &logWriter{}

type logWriter struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	file   *os.File
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buffer.Write(p)
	if w.buffer.Len() > maxMemoryLog {
		tail := w.buffer.Bytes()[w.buffer.Len()-maxMemoryLog/2:]
		if newline := bytes.IndexByte(tail, '\n'); newline >= 0 {
			tail = tail[newline+1:]
		} else {
			tail = nil
		}
		retained := append([]byte(nil), tail...)
		w.buffer.Reset()
		w.buffer.Write(retained)
	}
	if w.file != nil {
		if _, err := w.file.Write(p); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (w *logWriter) reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	w.buffer.Reset()
}

func (w *logWriter) setFile(enabled bool, directory string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !enabled {
		if w.file == nil {
			return nil
		}
		err := w.file.Close()
		w.file = nil
		return err
	}
	if w.file != nil {
		return nil
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}
	file, err := os.CreateTemp(directory, "Finicky-Plus_"+time.Now().Format("2006-01-02_15-04-05")+"_*.log")
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	if _, err = file.Write(w.buffer.Bytes()); err != nil {
		file.Close()
		return fmt.Errorf("failed to write buffered logs: %w", err)
	}
	w.file = file
	return nil
}
