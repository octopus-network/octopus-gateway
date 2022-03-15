package main

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger() {
	consoleConfig := zap.NewProductionEncoderConfig()
	consoleConfig.CallerKey = zapcore.OmitKey
	consoleEncoder := zapcore.NewJSONEncoder(consoleConfig)
	consoleSyncer := zapcore.Lock(os.Stdout)

	core := zapcore.NewCore(consoleEncoder, consoleSyncer, zap.NewAtomicLevel())
	logger := zap.New(core)
	defer logger.Sync()

	zap.ReplaceGlobals(logger)
}
