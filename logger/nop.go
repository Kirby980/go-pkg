package logger

type NopLogger struct{}

// With implements LoggerV1.
func (n *NopLogger) With(args ...Field) LoggerV1 {
	return n
}

func (n *NopLogger) Debug(msg string, fields ...Field) {
}
func (n *NopLogger) Info(msg string, fields ...Field) {
}
func (n *NopLogger) Warn(msg string, fields ...Field) {
}
func (n *NopLogger) Error(msg string, fields ...Field) {
}
