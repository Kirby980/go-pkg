package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type logger struct {
	*zap.SugaredLogger
}

var log *logger
var once sync.Once

func OnceLogger(name string) *logger {
	once.Do(func() {
		log = newLogger(true, name)
	})
	return log

}

func newLogger(release bool, scope string) *logger {
	debugPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.DebugLevel
	})

	var encoder zapcore.Encoder
	if release {
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)

	} else {
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	consoleStdout := zapcore.Lock(os.Stdout)

	zapCores := make([]zapcore.Core, 0)

	zapCores = append(zapCores, zapcore.NewCore(encoder, consoleStdout, debugPriority))
	coreTree := zapcore.NewTee(zapCores...)

	sugar := zap.New(coreTree).Sugar()
	return &logger{sugar}
}
