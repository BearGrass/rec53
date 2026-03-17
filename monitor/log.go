package monitor

import (
	"flag"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var logFile = flag.String("rec53.log", "./log/rec53.log", "Log file path")

var Rec53Log *zap.SugaredLogger

var atomicLevel = zap.NewAtomicLevelAt(zapcore.DebugLevel)

func InitLogger() {
	writeSyncer := getLogWriter()
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, atomicLevel)
	monitor := zap.New(core, zap.AddCaller())
	Rec53Log = monitor.Sugar()
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// getLogWriter returns a WriteSyncer for the configured log destination.
// When logFile is /dev/stderr or /dev/stdout, writes directly to the
// corresponding os.File to avoid lumberjack rotation on special files.
func getLogWriter() zapcore.WriteSyncer {
	switch *logFile {
	case "/dev/stderr":
		return zapcore.AddSync(os.Stderr)
	case "/dev/stdout":
		return zapcore.AddSync(os.Stdout)
	default:
		return zapcore.AddSync(&lumberjack.Logger{
			Filename:   *logFile,
			MaxSize:    1,
			MaxBackups: 5,
			MaxAge:     30,
			Compress:   false,
		})
	}
}

func SetLogLevel(level zapcore.Level) {
	atomicLevel.SetLevel(level)
}
