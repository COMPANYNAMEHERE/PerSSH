package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LogLevel defines the severity.
type LogLevel string

const (
	LogLevelInfo  LogLevel = "INFO"
	LogLevelError LogLevel = "ERROR"
	LogLevelAudit LogLevel = "AUDIT"
)

// Logger is a simple file logger.
type Logger struct {
	SystemLogFile *os.File
	AuditLogFile  *os.File
}

// NewLogger is an alias for InitLogger.
func NewLogger(sysLog, auditLog string) (*Logger, error) {
	return InitLogger()
}

// InitLogger opens log files.
func InitLogger() (*Logger, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(exePath)

	sysLog, err := os.OpenFile(filepath.Join(dir, "system.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	auditLog, err := os.OpenFile(filepath.Join(dir, "audit.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		sysLog.Close()
		return nil, err
	}

	return &Logger{
		SystemLogFile: sysLog,
		AuditLogFile:  auditLog,
	}, nil
}

func (l *Logger) Close() {
	if l.SystemLogFile != nil {
		l.SystemLogFile.Close()
	}
	if l.AuditLogFile != nil {
		l.AuditLogFile.Close()
	}
}

// Error logs technical errors.
func (l *Logger) Error(format string, v ...interface{}) {
	l.System(format, v...)
}

// System logs technical errors.
func (l *Logger) System(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(l.SystemLogFile, "[%s] [SYSTEM] %s\n", ts, msg)
}

// Audit logs user actions.
func (l *Logger) Audit(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(l.AuditLogFile, "[%s] [AUDIT] %s\n", ts, msg)
}

// EnsureDir checks if a directory exists, creating it if not.
func EnsureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}
