// Package logger provides the application's structured logging setup.
//
// It builds a zap.Logger that writes human-readable logs to stdout and
// JSON logs to a rotating file managed by lumberjack.
package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Rotation defaults for the file sink.
const (
	defaultMaxSizeMB  = 100
	defaultMaxBackups = 7
	defaultMaxAgeDays = 30
)

// New builds a zap.Logger that emits to both stdout (console encoder) and a
// rotating file (JSON encoder). The level is parsed from level (e.g. "debug",
// "info", "warn", "error"); path is the log file location.
func New(level, path string) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("logger: parse level %q: %w", level, err)
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("logger: create log directory %q: %w", dir, err)
		}
	}

	fileSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   path,
		MaxSize:    defaultMaxSizeMB,
		MaxBackups: defaultMaxBackups,
		MaxAge:     defaultMaxAgeDays,
		Compress:   true,
	})

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), lvl),
		zapcore.NewCore(fileEncoder, fileSyncer, lvl),
	)

	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)), nil
}
