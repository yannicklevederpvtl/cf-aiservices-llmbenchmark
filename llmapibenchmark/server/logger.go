package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
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

// LogContext provides context for log messages
type LogContext struct {
	JobID     string
	UserID    string
	RequestID string
	Model     string
	Operation string
}

// Logger provides structured logging with proper output streams
type Logger struct {
	debug *log.Logger
	info  *log.Logger
	warn  *log.Logger
	error *log.Logger
	fatal *log.Logger
	isCF  bool // Cloud Foundry environment
}

// JSONLogEntry represents a structured log entry for Cloud Foundry
type JSONLogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Context   *LogContext            `json:"context,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Global logger instance
var AppLogger *Logger

// NewLogger creates a new structured logger
func NewLogger() *Logger {
	// Check if running in Cloud Foundry
	isCF := os.Getenv("VCAP_APPLICATION") != ""
	
	// Normal logs (INFO, DEBUG, WARN) → stdout (white/green in CF)
	stdout := os.Stdout
	
	// Error logs (ERROR, FATAL) → stderr (red in CF)
	stderr := os.Stderr
	
	return &Logger{
		debug: log.New(stdout, "[DEBUG] ", log.LstdFlags|log.Lshortfile),
		info:  log.New(stdout, "[INFO]  ", log.LstdFlags|log.Lshortfile),
		warn:  log.New(stdout, "[WARN]  ", log.LstdFlags|log.Lshortfile),
		error: log.New(stderr, "[ERROR] ", log.LstdFlags|log.Lshortfile),
		fatal: log.New(stderr, "[FATAL] ", log.LstdFlags|log.Lshortfile),
		isCF:  isCF,
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	if l.isCF {
		l.logJSON(DEBUG, format, nil, nil, v...)
	} else {
		l.debug.Printf(format, v...)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	if l.isCF {
		l.logJSON(INFO, format, nil, nil, v...)
	} else {
		l.info.Printf(format, v...)
	}
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	if l.isCF {
		l.logJSON(WARN, format, nil, nil, v...)
	} else {
		l.warn.Printf(format, v...)
	}
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	if l.isCF {
		l.logJSON(ERROR, format, nil, nil, v...)
	} else {
		l.error.Printf(format, v...)
	}
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, v ...interface{}) {
	if l.isCF {
		l.logJSON(FATAL, format, nil, nil, v...)
	} else {
		l.fatal.Printf(format, v...)
	}
	os.Exit(1)
}

// DebugWithContext logs a debug message with context
func (l *Logger) DebugWithContext(ctx *LogContext, format string, v ...interface{}) {
	if l.isCF {
		l.logJSON(DEBUG, format, ctx, nil, v...)
	} else {
		prefix := l.formatContext(ctx)
		l.debug.Printf(prefix+format, v...)
	}
}

// DebugWithFields logs a debug message with structured fields
func (l *Logger) DebugWithFields(format string, fields map[string]interface{}, v ...interface{}) {
	if l.isCF {
		l.logJSON(DEBUG, format, nil, fields, v...)
	} else {
		fieldStr := l.formatFields(fields)
		l.debug.Printf(format+fieldStr, v...)
	}
}

// InfoWithContext logs an info message with context
func (l *Logger) InfoWithContext(ctx *LogContext, format string, v ...interface{}) {
	if l.isCF {
		l.logJSON(INFO, format, ctx, nil, v...)
	} else {
		prefix := l.formatContext(ctx)
		l.info.Printf(prefix+format, v...)
	}
}

// WarnWithContext logs a warning message with context
func (l *Logger) WarnWithContext(ctx *LogContext, format string, v ...interface{}) {
	if l.isCF {
		l.logJSON(WARN, format, ctx, nil, v...)
	} else {
		prefix := l.formatContext(ctx)
		l.warn.Printf(prefix+format, v...)
	}
}

// WarnWithFields logs a warning message with structured fields
func (l *Logger) WarnWithFields(format string, fields map[string]interface{}, v ...interface{}) {
	if l.isCF {
		l.logJSON(WARN, format, nil, fields, v...)
	} else {
		fieldStr := l.formatFields(fields)
		l.warn.Printf(format+fieldStr, v...)
	}
}

// ErrorWithContext logs an error message with context
func (l *Logger) ErrorWithContext(ctx *LogContext, format string, v ...interface{}) {
	if l.isCF {
		l.logJSON(ERROR, format, ctx, nil, v...)
	} else {
		prefix := l.formatContext(ctx)
		l.error.Printf(prefix+format, v...)
	}
}

// InfoWithFields logs an info message with structured fields
func (l *Logger) InfoWithFields(format string, fields map[string]interface{}, v ...interface{}) {
	if l.isCF {
		l.logJSON(INFO, format, nil, fields, v...)
	} else {
		fieldStr := l.formatFields(fields)
		l.info.Printf(format+fieldStr, v...)
	}
}

// ErrorWithFields logs an error message with structured fields
func (l *Logger) ErrorWithFields(format string, fields map[string]interface{}, v ...interface{}) {
	if l.isCF {
		l.logJSON(ERROR, format, nil, fields, v...)
	} else {
		fieldStr := l.formatFields(fields)
		l.error.Printf(format+fieldStr, v...)
	}
}

// logJSON logs a structured JSON message for Cloud Foundry
func (l *Logger) logJSON(level LogLevel, format string, ctx *LogContext, fields map[string]interface{}, v ...interface{}) {
	message := format
	if len(v) > 0 {
		message = fmt.Sprintf(format, v...)
	}
	
	entry := JSONLogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level.String(),
		Message:   message,
		Context:   ctx,
		Fields:    fields,
	}
	
	// Choose output stream based on level
	var output io.Writer
	if level >= ERROR {
		output = os.Stderr
	} else {
		output = os.Stdout
	}
	
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	encoder.Encode(entry)
}

// formatContext formats context for human-readable logs
func (l *Logger) formatContext(ctx *LogContext) string {
	if ctx == nil {
		return ""
	}
	
	parts := []string{}
	if ctx.JobID != "" {
		parts = append(parts, fmt.Sprintf("[Job:%s]", ctx.JobID))
	}
	if ctx.RequestID != "" {
		parts = append(parts, fmt.Sprintf("[Req:%s]", ctx.RequestID))
	}
	if ctx.Model != "" {
		parts = append(parts, fmt.Sprintf("[Model:%s]", ctx.Model))
	}
	if ctx.Operation != "" {
		parts = append(parts, fmt.Sprintf("[Op:%s]", ctx.Operation))
	}
	
	if len(parts) > 0 {
		return fmt.Sprintf("%s ", fmt.Sprintf("%s", parts))
	}
	return ""
}

// formatFields formats structured fields for human-readable logs
func (l *Logger) formatFields(fields map[string]interface{}) string {
	if len(fields) == 0 {
		return ""
	}
	
	fieldStr := " |"
	for k, v := range fields {
		fieldStr += fmt.Sprintf(" %s=%v", k, v)
	}
	return fieldStr
}

// WithContext returns a context logger for chaining
func (l *Logger) WithContext(ctx *LogContext) *ContextLogger {
	return &ContextLogger{
		logger: l,
		ctx:    ctx,
	}
}

// ContextLogger provides context-aware logging
type ContextLogger struct {
	logger *Logger
	ctx    *LogContext
}

// Debug logs a debug message with the context
func (cl *ContextLogger) Debug(format string, v ...interface{}) {
	cl.logger.DebugWithContext(cl.ctx, format, v...)
}

// Info logs an info message with the context
func (cl *ContextLogger) Info(format string, v ...interface{}) {
	cl.logger.InfoWithContext(cl.ctx, format, v...)
}

// Warn logs a warning message with the context
func (cl *ContextLogger) Warn(format string, v ...interface{}) {
	cl.logger.WarnWithContext(cl.ctx, format, v...)
}

// Error logs an error message with the context
func (cl *ContextLogger) Error(format string, v ...interface{}) {
	cl.logger.ErrorWithContext(cl.ctx, format, v...)
}

// InfoWithFields logs an info message with context and fields
func (cl *ContextLogger) InfoWithFields(format string, fields map[string]interface{}, v ...interface{}) {
	if cl.logger.isCF {
		cl.logger.logJSON(INFO, format, cl.ctx, fields, v...)
	} else {
		prefix := cl.logger.formatContext(cl.ctx)
		fieldStr := cl.logger.formatFields(fields)
		cl.logger.info.Printf(prefix+format+fieldStr, v...)
	}
}

// ErrorWithFields logs an error message with context and fields
func (cl *ContextLogger) ErrorWithFields(format string, fields map[string]interface{}, v ...interface{}) {
	if cl.logger.isCF {
		cl.logger.logJSON(ERROR, format, cl.ctx, fields, v...)
	} else {
		prefix := cl.logger.formatContext(cl.ctx)
		fieldStr := cl.logger.formatFields(fields)
		cl.logger.error.Printf(prefix+format+fieldStr, v...)
	}
}
