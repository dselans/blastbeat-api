package clog

import (
	"sync"

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
	fields    map[string]zap.Field
	fieldsMtx *sync.Mutex
	logger    *zap.Logger
}

func New(logger *zap.Logger, fields ...zap.Field) ICustomLog {
	tmpFields := make(map[string]zap.Field)

	if logger == nil {
		logger = zap.NewNop()
	}

	mtx := &sync.Mutex{}

	return &CustomLog{
		logger:    logger,
		fieldsMtx: mtx, // We need a mutex to avoid concurrent map write panics
		fields:    UpdateMap(mtx, tmpFields, fields...),
	}
}

func (c CustomLog) Debug(msg string, fields ...zap.Field) {
	c.logger.Debug(msg, append(MapToFields(c.fieldsMtx, c.fields), fields...)...)
}

func (c CustomLog) Info(msg string, fields ...zap.Field) {
	c.logger.Info(msg, append(MapToFields(c.fieldsMtx, c.fields), fields...)...)
}

func (c CustomLog) Warn(msg string, fields ...zap.Field) {
	c.logger.Warn(msg, append(MapToFields(c.fieldsMtx, c.fields), fields...)...)
}

func (c CustomLog) Error(msg string, fields ...zap.Field) {
	c.logger.Error(msg, append(MapToFields(c.fieldsMtx, c.fields), fields...)...)
}

func (c CustomLog) Fatal(msg string, fields ...zap.Field) {
	c.logger.Fatal(msg, append(MapToFields(c.fieldsMtx, c.fields), fields...)...)
}

func (c CustomLog) With(fields ...zap.Field) ICustomLog {
	newFields := make(map[string]zap.Field)

	c.fieldsMtx.Lock()
	for k, v := range c.fields {
		newFields[k] = v
	}
	c.fieldsMtx.Unlock()

	return New(c.logger, MapToFields(nil, UpdateMap(&sync.Mutex{}, newFields, fields...))...)
}

func UpdateMap(mtx *sync.Mutex, m map[string]zap.Field, f ...zap.Field) map[string]zap.Field {
	if mtx != nil {
		mtx.Lock()
		defer mtx.Unlock()
	}

	for _, field := range f {
		m[field.Key] = field
	}

	return m
}

func MapToFields(mtx *sync.Mutex, m map[string]zap.Field) []zap.Field {
	if mtx != nil {
		mtx.Lock()
		defer mtx.Unlock()
	}

	fields := make([]zap.Field, 0, len(m))

	for _, field := range m {
		fields = append(fields, field)
	}

	return fields
}
