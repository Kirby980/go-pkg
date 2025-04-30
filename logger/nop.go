package logger

type NopLogger struct{}

func (n *NopLogger) Debug(msg string, fields ...Field) {
}
func (n *NopLogger) Info(msg string, fields ...Field) {
}
func (n *NopLogger) Warn(msg string, fields ...Field) {
}
func (n *NopLogger) Error(msg string, fields ...Field) {
}
