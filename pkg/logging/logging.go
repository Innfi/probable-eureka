package logging

import (
	"log/slog"
	"os"
	"path/filepath"
)

const (
	defaultLogPath = "/var/log/cni/test-cni.log"
)

var Logger *slog.Logger

func Init(logPath string) error {
	if logPath == "" {
		logPath = defaultLogPath
	}

	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	Logger = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With("plugin", "test-cni")

	return nil
}

func InitStderr() {
	Logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With("plugin", "test-cni")
}
