package logger

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// LogLevel defines the log level
type LogLevel int

const (
	// LogLevelDebug debug level
	LogLevelDebug LogLevel = iota
	// LogLevelInfo info level
	LogLevelInfo
	// LogLevelError error level
	LogLevelError
)

var (
	// currentLogLevel is the current log level (default to Error for production)
	currentLogLevel = LogLevelError
	// errorLogger logs error messages
	errorLogger = log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lshortfile)
	// infoLogger logs info messages
	infoLogger = log.New(os.Stdout, "[INFO] ", log.LstdFlags|log.Lshortfile)
	// debugLogger logs debug messages
	debugLogger = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags|log.Lshortfile)
)

func init() {
	// Initialize log level from LOG_LEVEL environment variable if set (highest priority)
	if logLevelEnv := os.Getenv("LOG_LEVEL"); logLevelEnv != "" {
		SetLogLevelFromString(logLevelEnv)
		return
	}

	// Otherwise, determine log level based on RUN_LEVEL
	runLevel := os.Getenv("RUN_LEVEL")
	switch strings.ToLower(runLevel) {
	case "local":
		SetLogLevel(LogLevelDebug)
	case "stage":
		SetLogLevel(LogLevelInfo)
	case "production":
		SetLogLevel(LogLevelError)
	default:
		// Default to error for production safety if RUN_LEVEL is not set or invalid
		SetLogLevel(LogLevelError)
		if runLevel == "" {
			infoLogger.Output(2, fmt.Sprintf("RUN_LEVEL not set, using default log level: %s", getLogLevelString(currentLogLevel)))
		} else {
			infoLogger.Output(2, fmt.Sprintf("Invalid RUN_LEVEL: %s, using default log level: %s", runLevel, getLogLevelString(currentLogLevel)))
		}
	}
}

// getLogLevelString returns the string representation of log level
func getLogLevelString(level LogLevel) string {
	switch level {
	case LogLevelDebug:
		return "debug"
	case LogLevelInfo:
		return "info"
	case LogLevelError:
		return "error"
	default:
		return "error"
	}
}

// SetLogLevel sets the global log level
func SetLogLevel(level LogLevel) {
	currentLogLevel = level
}

// SetLogLevelFromString sets the log level from string (debug, info, error)
func SetLogLevelFromString(levelStr string) {
	switch strings.ToLower(levelStr) {
	case "debug":
		SetLogLevel(LogLevelDebug)
	case "info":
		SetLogLevel(LogLevelInfo)
	case "error":
		SetLogLevel(LogLevelError)
	default:
		SetLogLevel(LogLevelError) // Default to error for production
	}
}

// Error logs an error message (always logged)
func Error(format string, v ...interface{}) {
	errorLogger.Output(2, fmt.Sprintf(format, v...))
}

// Info logs an info message (logged when level is Info or Debug)
func Info(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelInfo {
		infoLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Debug logs a debug message (only logged when level is Debug)
func Debug(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelDebug {
		debugLogger.Output(2, fmt.Sprintf(format, v...))
	}
}

// Errorf logs an error message with format
func Errorf(format string, v ...interface{}) {
	Error(format, v...)
}

// Infof logs an info message with format
func Infof(format string, v ...interface{}) {
	Info(format, v...)
}

// Debugf logs a debug message with format
func Debugf(format string, v ...interface{}) {
	Debug(format, v...)
}
