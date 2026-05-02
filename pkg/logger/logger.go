package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	instance *zap.Logger
	once     sync.Once
)

func Init(level string, jsonFormat bool) {
	once.Do(func() {
		var lvl zapcore.Level
		if err := lvl.UnmarshalText([]byte(level)); err != nil {
			lvl = zapcore.InfoLevel
		}

		encCfg := zap.NewProductionEncoderConfig()
		encCfg.TimeKey = "ts"
		encCfg.EncodeTime = zapcore.RFC3339NanoTimeEncoder
		encCfg.EncodeLevel = zapcore.CapitalLevelEncoder

		var encoder zapcore.Encoder
		if jsonFormat {
			encoder = zapcore.NewJSONEncoder(encCfg)
		} else {
			encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
			encoder = zapcore.NewConsoleEncoder(encCfg)
		}

		core := zapcore.NewCore(
			encoder,
			zapcore.AddSync(os.Stdout),
			zap.NewAtomicLevelAt(lvl),
		)

		instance = zap.New(core,
			zap.AddCaller(),
			zap.AddCallerSkip(0),
			zap.AddStacktrace(zapcore.ErrorLevel),
			zap.Fields(
				zap.String("component", "smartscaler"),
			),
		)
	})
}

func L() *zap.Logger {
	if instance == nil {
		Init("info", false)
	}
	return instance
}

func Sync() {
	if instance != nil {
		_ = instance.Sync()
	}
}
