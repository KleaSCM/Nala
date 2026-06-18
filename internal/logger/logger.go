/**
 * Structured logging wrapper around zap.
 * zapをラップした構造化ロガーね。
 *
 * JSON output to stdout or file with rotation.
 * JSON形式でstdoutかファイルに出力して、ローテーションも対応してるの。
 *
 * Author: KleaSCM
 * Email: KleaSCM@gmail.com
 */

package logger

import (
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	inner *zap.SugaredLogger
}

func New(level string, filePath string, maxSize int, maxAge int) (*Logger, error) {
	var lvl zapcore.Level
	if err := lvl.Set(level); err != nil {
		lvl = zapcore.InfoLevel
	}

	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var writer io.Writer
	writer = os.Stdout

	if filePath != "" {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err == nil {
			writer = io.MultiWriter(writer, &lumberjack.Logger{
				Filename:   filePath,
				MaxSize:    maxSize,
				MaxBackups: 5,
				MaxAge:     maxAge,
				Compress:   true,
			})
		}
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(writer),
		lvl,
	)

	inner := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return &Logger{inner: inner.Sugar()}, nil
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	l.inner.Debugw(msg, args...)
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.inner.Infow(msg, args...)
}

func (l *Logger) Warn(msg string, args ...interface{}) {
	l.inner.Warnw(msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.inner.Errorw(msg, args...)
}

func (l *Logger) With(fields ...interface{}) *Logger {
	return &Logger{inner: l.inner.With(fields...)}
}

func (l *Logger) Sync() error {
	return l.inner.Sync()
}
