// Package logger provides logging support to the application.
package logger

import (
	"io"
	"os"
	"strings"

	"github.com/dronm/elkonexport/config"
	"github.com/sirupsen/logrus"
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

func Initialize(logLevel string, logFileName string, rotation ...config.LogRotation) error {
	options := config.LogRotation{Compress: true}
	if len(rotation) > 0 {
		options = rotation[0]
	}
	options, err := options.WithDefaults()
	if err != nil {
		return err
	}
	newLogger := logrus.New()
	var output io.Closer
	// Close and join the previous writer before opening the same path again.
	FlushLogFile()

	if logFileName != "" {
		writer, err := newTimedWriter(logFileName, options)
		if err != nil {
			return err
		}
		output = writer
		newLogger.SetOutput(writer)
	}

	newLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	newLogger.SetLevel(logrusLogLevel(logLevel))
	Logger = newLogger
	logOutput = output

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
		if Logger != nil {
			Logger.SetOutput(os.Stderr)
		}
		_ = logOutput.Close()
		logOutput = nil
	}
}
