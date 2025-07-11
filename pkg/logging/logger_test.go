package logging

import (
	"testing"
)

func TestLoggingLevels(t *testing.T) {
	Info("info message")
	Warn("warn message")
	Error("error message")
	Debug("debug message")
}

func TestLoggingWithFields(t *testing.T) {
	Info("info with fields", "foo", 123, "bar", true)
	Error("error with fields", "err", "fail")
}
