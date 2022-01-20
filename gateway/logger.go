package main

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func InitLogger() {
	consoleEncoder := zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	consoleSyncer := zapcore.Lock(os.Stdout)

	prodConfig := zap.NewProductionEncoderConfig()
	prodConfig.CallerKey = zapcore.OmitKey
	jsonEncoder := zapcore.NewJSONEncoder(prodConfig)
	rotationSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "./logs/gateway.log",
		MaxSize:    100,
		MaxBackups: 5,
		MaxAge:     10,
	})

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, consoleSyncer, zap.NewAtomicLevel()),
		zapcore.NewCore(jsonEncoder, rotationSyncer, zapcore.InfoLevel),
	)

	logger := zap.New(core)
	defer logger.Sync()

	zap.ReplaceGlobals(logger)
}
