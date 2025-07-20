package logger

import (
	"time"

	"github.com/rvarun11/sqlite-mcp/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new zap sugared logger instance
func NewLogger(cfg *config.Config) (*zap.SugaredLogger, error) {
	loggerConfig := buildConfig(cfg)
	logger, err := loggerConfig.Build()
	if err != nil {
		return nil, err
	}
	return logger.Sugar(), nil
}

// NewTestLogger is a helper for tests
func NewTestLogger() *zap.SugaredLogger {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, _ := config.Build()
	return logger.Sugar()
}

// buildConfig returns the base configuration
func buildConfig(appConfig *config.Config) zap.Config {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.Kitchen)
	config.OutputPaths = []string{"stderr"} // preferred for mcps

	if appConfig.Debug {
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	return config
}
