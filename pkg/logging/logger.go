package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Logger *logrus.Logger

func init() {
	Logger = logrus.New()
	Logger.SetOutput(os.Stdout)
	Logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Set log level based on environment
	switch os.Getenv("LOG_LEVEL") {
	case "debug":
		Logger.SetLevel(logrus.DebugLevel)
	case "info":
		Logger.SetLevel(logrus.InfoLevel)
	case "warn":
		Logger.SetLevel(logrus.WarnLevel)
	case "error":
		Logger.SetLevel(logrus.ErrorLevel)
	default:
		Logger.SetLevel(logrus.InfoLevel)
	}
}

// Helper functions for different log levels
func Debug(args ...interface{}) {
	Logger.Debug(args...)
}

func Info(args ...interface{}) {
	Logger.Info(args...)
}

func Warn(args ...interface{}) {
	Logger.Warn(args...)
}

func Error(args ...interface{}) {
	Logger.Error(args...)
}

func Fatal(args ...interface{}) {
	Logger.Fatal(args...)
}

// WithFields adds fields to the log entry
func WithFields(fields map[string]interface{}) *logrus.Entry {
	return Logger.WithFields(fields)
}
