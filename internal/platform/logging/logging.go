package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

func New(filename, level string) (*logrus.Logger, io.Closer, error) {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetOutput(os.Stdout)
	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, nil, fmt.Errorf("parse log level: %w", err)
	}
	logger.SetLevel(parsedLevel)
	if filename == "" {
		return logger, nil, nil
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o750); err != nil {
		return nil, nil, fmt.Errorf("create log directory: %w", err)
	}
	file := &lumberjack.Logger{Filename: filename, MaxSize: 100, MaxBackups: 3, MaxAge: 28, Compress: true}
	logger.SetOutput(io.MultiWriter(os.Stdout, file))
	return logger, file, nil
}
