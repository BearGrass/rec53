package monitor

import (
	"flag"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logFile = flag.String("rec53.log", "/home/long/log/rec53.log", "Log file path")

var Rec53Log *zap.SugaredLogger

func InitLogger() {
	writeSyncer := getLogWriter()
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)
	monitor := zap.New(core, zap.AddCaller())
	Rec53Log = monitor.Sugar()
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getLogWriter() zapcore.WriteSyncer {
	lumberJackmonitor := &lumberjack.Logger{
		Filename:   *logFile,
		MaxSize:    1,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   false,
	}
	return zapcore.AddSync(lumberJackmonitor)
}

func SetLogLevel(level zapcore.Level) {
	Rec53Log.Desugar().Core().Enabled(level)
}
