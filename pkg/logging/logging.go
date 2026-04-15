package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents log severity level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Entry represents a structured log entry
type Entry struct {
	Timestamp time.Time              `json:"ts"`
	Level     Level                  `json:"level"`
	Message   string                 `json:"msg"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Logger provides structured logging
type Logger struct {
	level  Level
	output io.Writer
	mu     sync.Mutex
	fields map[string]interface{}
}

// New creates a new logger
func New(level Level, output io.Writer) *Logger {
	if output == nil {
		output = os.Stdout
	}
	return &Logger{
		level:  level,
		output: output,
		fields: make(map[string]interface{}),
	}
}

// WithField returns a new logger with additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value
	return &Logger{
		level:  l.level,
		output: l.output,
		fields: newFields,
	}
}

// WithFields returns a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newFields := make(map[string]interface{})
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	return &Logger{
		level:  l.level,
		output: l.output,
		fields: newFields,
	}
}

// log writes a log entry
func (l *Logger) log(level Level, msg string) {
	if level < l.level {
		return
	}

	entry := Entry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   msg,
		Fields:    l.fields,
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Use JSON encoding for structured logging
	encoder := json.NewEncoder(l.output)
	encoder.Encode(entry)
}

// Debug logs debug message
func (l *Logger) Debug(msg string) {
	l.log(DEBUG, msg)
}

// Debugf logs formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...))
}

// Info logs info message
func (l *Logger) Info(msg string) {
	l.log(INFO, msg)
}

// Infof logs formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, args...))
}

// Warn logs warning message
func (l *Logger) Warn(msg string) {
	l.log(WARN, msg)
}

// Warnf logs formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, args...))
}

// Error logs error message
func (l *Logger) Error(msg string) {
	l.log(ERROR, msg)
}

// Errorf logs formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...))
}

// Fatal logs fatal message and exits
func (l *Logger) Fatal(msg string) {
	l.log(FATAL, msg)
	os.Exit(1)
}

// Fatalf logs formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, args...))
	os.Exit(1)
}

// SetLevel updates the log level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// ParseLevel parses level string
func ParseLevel(s string) Level {
	switch s {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// Global logger instance
var defaultLogger = New(INFO, os.Stdout)

// SetDefault sets the global logger
func SetDefault(l *Logger) {
	defaultLogger = l
}

// GetDefault returns the global logger
func GetDefault() *Logger {
	return defaultLogger
}

// Package-level convenience functions
func Debug(msg string)                  { defaultLogger.Debug(msg) }
func Debugf(f string, a ...interface{}) { defaultLogger.Debugf(f, a...) }
func Info(msg string)                   { defaultLogger.Info(msg) }
func Infof(f string, a ...interface{})  { defaultLogger.Infof(f, a...) }
func Warn(msg string)                   { defaultLogger.Warn(msg) }
func Warnf(f string, a ...interface{})  { defaultLogger.Warnf(f, a...) }
func Error(msg string)                  { defaultLogger.Error(msg) }
func Errorf(f string, a ...interface{}) { defaultLogger.Errorf(f, a...) }
func Fatal(msg string)                  { defaultLogger.Fatal(msg) }
func Fatalf(f string, a ...interface{}) { defaultLogger.Fatalf(f, a...) }

// WithField returns logger with field
func WithField(key string, value interface{}) *Logger {
	return defaultLogger.WithField(key, value)
}

// WithFields returns logger with fields
func WithFields(fields map[string]interface{}) *Logger {
	return defaultLogger.WithFields(fields)
}
