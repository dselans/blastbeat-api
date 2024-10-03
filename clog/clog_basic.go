package clog

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// This is used as a "fallback" logger in situations where we do not have a zap
// logger. This logger prints log messages to stdout with the following format:
//
// 2024-10-20 01:02:03 [LEVEL] Example message {"key1": "value1", "key2": "value2"}

const (
	timeFormat = "2006-01-02 15:04:05"
)

type CustomLogBasic struct {
	fields map[string]zap.Field
	mtx    *sync.Mutex
}

func NewBasic(fields ...zap.Field) ICustomLog {
	tmpFields := make(map[string]zap.Field)

	mtx := &sync.Mutex{}

	return &CustomLogBasic{
		mtx:    mtx,
		fields: UpdateMap(mtx, tmpFields, fields...),
	}
}

func (c CustomLogBasic) Debug(msg string, fields ...zap.Field) {
	date := time.Now().UTC().Format(timeFormat)
	fmt.Printf("%s [DEBUG] %s {%s}\n", date, msg, FieldsToString(MapToFields(c.mtx, UpdateMap(c.mtx, c.fields, fields...))))
}

func (c CustomLogBasic) Info(msg string, fields ...zap.Field) {
	date := time.Now().UTC().Format(timeFormat)
	fmt.Printf("%s [INFO] %s {%s}\n", date, msg, FieldsToString(MapToFields(c.mtx, UpdateMap(c.mtx, c.fields, fields...))))
}

func (c CustomLogBasic) Warn(msg string, fields ...zap.Field) {
	date := time.Now().UTC().Format(timeFormat)
	fmt.Printf("%s [WARN] %s {%s}\n", date, msg, FieldsToString(MapToFields(c.mtx, UpdateMap(c.mtx, c.fields, fields...))))
}

func (c CustomLogBasic) Error(msg string, fields ...zap.Field) {
	date := time.Now().UTC().Format(timeFormat)
	fmt.Printf("%s [ERROR] %s {%s}\n", date, msg, FieldsToString(MapToFields(c.mtx, UpdateMap(c.mtx, c.fields, fields...))))
}

func (c CustomLogBasic) Fatal(msg string, fields ...zap.Field) {
	date := time.Now().UTC().Format(timeFormat)
	fmt.Printf("%s [FATAL] %s {%s}\n", date, msg, FieldsToString(MapToFields(c.mtx, UpdateMap(c.mtx, c.fields, fields...))))
}

func (c CustomLogBasic) With(fields ...zap.Field) ICustomLog {
	return NewBasic(MapToFields(c.mtx, UpdateMap(c.mtx, c.fields, fields...))...)
}

func FieldsToString(fields []zap.Field) string {
	var fieldString string

	for _, f := range fields {
		// Example: 2024-10-20 01:02:03 [LEVEL] Example message {"key1": "value1", "key2": "value2"}
		fieldString += fmt.Sprintf(`"%s": "%s", `, f.Key, f.String)
	}

	return strings.TrimSuffix(fieldString, ", ")
}
