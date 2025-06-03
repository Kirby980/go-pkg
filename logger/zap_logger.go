package logger

import (
	"go.uber.org/zap"
)

// 适配器模式
// ZapLogger 是一个实现了 Logger 接口的结构体
type ZapLogger struct {
	l *zap.Logger
}

// With implements Logger.
func (z *ZapLogger) With(args ...Field) Logger {
	return &ZapLogger{
		l: z.l.With(z.toZapFields(args)...),
	}
}

func NewZapLogger(l *zap.Logger) Logger {
	return &ZapLogger{
		l: l,
	}
}

func (z *ZapLogger) Debug(msg string, args ...Field) {
	z.l.Debug(msg, z.toZapFields(args)...)
}
func (z *ZapLogger) Info(msg string, args ...Field) {
	z.l.Info(msg, z.toZapFields(args)...)
}
func (z *ZapLogger) Warn(msg string, args ...Field) {
	z.l.Warn(msg, z.toZapFields(args)...)
}
func (z *ZapLogger) Error(msg string, args ...Field) {
	z.l.Error(msg, z.toZapFields(args)...)
}

// 一个key 一个value
func (z *ZapLogger) toZapFields(args []Field) []zap.Field {
	fields := make([]zap.Field, 0, len(args))
	for _, v := range args {
		fields = append(fields, zap.Any(v.Key, v.Value))
	}
	return fields
}
