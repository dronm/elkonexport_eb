// Package logger provides logging support to the application.
package logger

import (
	"io"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger is the global logger instance.
var Logger *logrus.Logger

var logOutput io.Closer

const (
	logLevelDebug = "debug"
	logLevelInfo  = "info"
	logLevelWarn  = "warn"
	logLevelError = "error"
)

type LogWriter struct {
	logger *logrus.Logger
}

func NewLogWriter() *LogWriter {
	return &LogWriter{logger: Logger}
}

func (lw *LogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		lw.logger.Info(msg)
	}

	return len(p), nil
}

func Initialize(logLevel string, logFileName string) error {
	Logger = logrus.New()

	if logFileName != "" {
		writer := &lumberjack.Logger{
			Filename:   logFileName,
			MaxSize:    20,   // megabytes
			MaxBackups: 5,    // keep last 5 rotated files
			MaxAge:     30,   // days
			LocalTime:  true, // use local time in rotated filenames
			Compress:   true, // gzip old rotated files
		}

		logOutput = writer
		Logger.SetOutput(writer)
	}

	Logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	Logger.SetLevel(logrusLogLevel(logLevel))

	return nil
}

func SetLogLevel(logLevel string) {
	if Logger == nil {
		return
	}

	Logger.SetLevel(logrusLogLevel(logLevel))
}

func logrusLogLevel(logLevel string) logrus.Level {
	switch logLevel {
	case logLevelDebug:
		return logrus.DebugLevel
	case logLevelInfo:
		return logrus.InfoLevel
	case logLevelWarn:
		return logrus.WarnLevel
	case logLevelError:
		return logrus.ErrorLevel
	default:
		return logrus.DebugLevel
	}
}

func FlushLogFile() {
	if logOutput != nil {
		_ = logOutput.Close()
		logOutput = nil
	}
}
