package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
}

// Field aliases for cleaner usage
type Field = zap.Field

// New creates a new logger with the specified level
func New(level string) Logger {
	var zapLevel zapcore.Level
	switch level {
	case "debug":
		zapLevel = zapcore.DebugLevel
	case "info":
		zapLevel = zapcore.InfoLevel
	case "warn":
		zapLevel = zapcore.WarnLevel
	case "error":
		zapLevel = zapcore.ErrorLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      zapLevel == zapcore.DebugLevel,
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	// En desarrollo, usar formato más legible
	if zapLevel == zapcore.DebugLevel {
		config.Encoding = "console"
		config.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	}

	logger, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		panic(err)
	}

	return Logger{logger}
}

// With creates a child logger with the given fields
func (l Logger) With(fields ...Field) Logger {
	return Logger{l.Logger.With(fields...)}
}

// String creates a string field
func String(key, val string) Field {
	return zap.String(key, val)
}

// Int creates an int field
func Int(key string, val int) Field {
	return zap.Int(key, val)
}

// Error creates an error field
func Error(err error) Field {
	return zap.Error(err)
}

// Bool creates a bool field
func Bool(key string, val bool) Field {
	return zap.Bool(key, val)
}

// Any creates a field with any value
func Any(key string, val interface{}) Field {
	return zap.Any(key, val)
}
