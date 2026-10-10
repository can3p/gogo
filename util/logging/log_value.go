package logging

import "go.uber.org/zap"

// LogValue logs a at debug level under msg and returns it, to inspect a
// value in the middle of an expression.
func LogValue[A any](logger *zap.Logger, msg string, a A) A {
	logger.Debug(msg, zap.Any("value", a))

	return a
}
