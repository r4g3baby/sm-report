package config

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
)

var Logger *zap.SugaredLogger

func init() {
	var zapConfig = zap.NewProductionEncoderConfig()
	var level = zap.InfoLevel
	if Config.Debug {
		level = zap.DebugLevel
	}

	zapConfig.ConsoleSeparator = "\u0020"
	zapConfig.EncodeTime = zapcore.TimeEncoderOfLayout("02 Jan 15:04")
	zapConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(zapConfig)

	var core zapcore.Core
	if Config.Logger.Enabled {
		zapConfig.EncodeTime = zapcore.EpochNanosTimeEncoder
		zapConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
		fileEncoder := zapcore.NewJSONEncoder(zapConfig)

		core = zapcore.NewTee(
			zapcore.NewCore(fileEncoder, zapcore.AddSync(&lumberjack.Logger{
				Filename:   Config.Logger.Filename,
				MaxSize:    Config.Logger.MaxSize,
				MaxAge:     Config.Logger.MaxAge,
				MaxBackups: Config.Logger.MaxBackups,
				LocalTime:  Config.Logger.LocalTime,
				Compress:   Config.Logger.Compress,
			}), level),
			zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stderr), level),
		)
	} else {
		core = zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stderr), level)
	}

	Logger = zap.New(core).Sugar()
}
