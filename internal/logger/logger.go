package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger
var Sugar *zap.SugaredLogger

func Initialize(mode string) error {
	var config zap.Config

	if mode == "release" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	var err error
	Log, err = config.Build()
	if err != nil {
		return err
	}

	Sugar = Log.Sugar()
	return nil
}

// Sync flushes any buffered log entries.
// Errors are ignored because Sync often returns EINVAL on stdout/stderr
// (common on Linux containers and macOS).
func Sync() {
	if Log != nil {
		_ = Log.Sync()
	}
}

func Info(msg string, fields ...zap.Field) {
	if Log != nil {
		Log.Info(msg, fields...)
	}
}

func Error(msg string, fields ...zap.Field) {
	if Log != nil {
		Log.Error(msg, fields...)
	}
}

func Debug(msg string, fields ...zap.Field) {
	if Log != nil {
		Log.Debug(msg, fields...)
	}
}

func Warn(msg string, fields ...zap.Field) {
	if Log != nil {
		Log.Warn(msg, fields...)
	}
}

// Fatal logs a message then exits. Unlike zap's Fatal which calls os.Exit
// directly (bypassing defers), this ensures logs are flushed first.
func Fatal(msg string, fields ...zap.Field) {
	if Log != nil {
		Log.Error(msg, fields...)
		Sync()
	}
	os.Exit(1)
}

func With(fields ...zap.Field) *zap.Logger {
	if Log != nil {
		return Log.With(fields...)
	}
	return nil
}

func GetGinLogger() *zap.Logger {
	return Log
}

// Named returns a named logger for subsystem identification.
func Named(name string) *zap.Logger {
	if Log != nil {
		return Log.Named(name)
	}
	return nil
}

// L returns the underlying zap logger.
func L() *zap.Logger {
	return Log
}

func init() {
	// Initialize with development config by default
	// This will be overwritten by Initialize() call in main
	mode := os.Getenv("GIN_MODE")
	if mode == "" {
		mode = "debug"
	}
	_ = Initialize(mode)
}
