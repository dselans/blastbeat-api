package clog

import (
	"go.uber.org/zap"
)

// This exists to support dependency injection

type CustomLogNoop struct{}

func (c CustomLogNoop) Debug(_ string, _ ...zap.Field) {}

func (c CustomLogNoop) Info(_ string, _ ...zap.Field) {}

func (c CustomLogNoop) Warn(_ string, _ ...zap.Field) {}

func (c CustomLogNoop) Error(_ string, _ ...zap.Field) {}

func (c CustomLogNoop) Fatal(_ string, _ ...zap.Field) {}

func (c CustomLogNoop) With(_ ...zap.Field) ICustomLog { return &CustomLogNoop{} }
