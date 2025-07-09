package clog

import (
	"go.uber.org/zap"
)

type TestLogger struct {
	Messages []string
}

func (t *TestLogger) Debug(msg string, _ ...zap.Field) { t.append("DEBUG: " + msg) }

func (t *TestLogger) Info(msg string, _ ...zap.Field) { t.append("INFO: " + msg) }

func (t *TestLogger) Warn(msg string, _ ...zap.Field) { t.append("WARN: " + msg) }

func (t *TestLogger) Error(msg string, _ ...zap.Field) { t.append("ERROR: " + msg) }

func (t *TestLogger) Fatal(msg string, _ ...zap.Field) { t.append("FATA: " + msg) }

func (t *TestLogger) With(_ ...zap.Field) ICustomLog { return t }

func (t *TestLogger) append(msg string) {
	if t.Messages == nil {
		t.Messages = make([]string, 0)
	}

	t.Messages = append(t.Messages, msg)
}
