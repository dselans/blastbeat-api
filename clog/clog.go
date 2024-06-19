package clog

import (
	"go.uber.org/zap"
)

// This package is a simple wrapper around zap.Logger that supports including
// initial fields in the logger (ie. .With(...)). NR's zap integration only
// includes attributes that are added to the logger at the time of the log call.
//
// This allows us to include "top-level" attributes like "env", "pkg", "method",
// etc. into all log messages without having to tweak/adjust the zap's core or
// logger.

type ICustomLog interface {
	Debug(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
	Error(msg string, fields ...zap.Field)
	Fatal(msg string, fields ...zap.Field)
	With(fields ...zap.Field) ICustomLog
}

type CustomLog struct {
	fields []zap.Field
	logger *zap.Logger
}

func New(logger *zap.Logger, fields ...zap.Field) ICustomLog {
	tmpFields := make([]zap.Field, 0)

	if logger == nil {
		logger = zap.NewNop()
	}

	return &CustomLog{
		logger: logger,
		fields: append(tmpFields, fields...),
	}
}

func (c CustomLog) Debug(msg string, fields ...zap.Field) {
	fields = append(c.fields, fields...)
	c.logger.Debug(msg, fields...)
}

func (c CustomLog) Info(msg string, fields ...zap.Field) {
	fields = append(c.fields, fields...)
	c.logger.Info(msg, fields...)
}

func (c CustomLog) Warn(msg string, fields ...zap.Field) {
	fields = append(c.fields, fields...)
	c.logger.Warn(msg, fields...)
}

func (c CustomLog) Error(msg string, fields ...zap.Field) {
	fields = append(c.fields, fields...)
	c.logger.Error(msg, fields...)
}

func (c CustomLog) Fatal(msg string, fields ...zap.Field) {
	fields = append(c.fields, fields...)
	c.logger.Fatal(msg, fields...)
}

func (c CustomLog) With(fields ...zap.Field) ICustomLog {
	fields = append(c.fields, fields...)
	return New(c.logger, fields...)
}
