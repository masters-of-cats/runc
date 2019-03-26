package logs

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
)

// loggingConfigured will be set once logging has been configured via invoking `ConfigureLogging`.
// Subsequent invocations of `ConfigureLogging` would be no-op
var loggingConfigured = false

type LoggingConfiguration struct {
	IsDebug     bool
	LogFormat   string
	LogFilePath string
	LogPipeFd   string
}

func ForwardLogs(p *os.File) {
	defer p.Close()

	type jsonLog struct {
		Level string `json:"level"`
		Msg   string `json:"msg"`
	}

	dec := json.NewDecoder(p)
	for {
		var jl jsonLog
		if err := dec.Decode(&jl); err != nil {
			if err == io.EOF {
				logrus.Debug("child pipe closed")
				return
			}
			logrus.Errorf("json logs decoding error: %+v", err)
			return
		}

		lvl, err := logrus.ParseLevel(jl.Level)
		if err != nil {
			fmt.Printf("parsing error\n")
		}
		logFunc(lvl)(jl.Msg)
	}
}

func logFunc(level logrus.Level) func(args ...interface{}) {
	switch level {
	case logrus.PanicLevel:
		return logrus.Panic
	case logrus.FatalLevel:
		return logrus.Fatal
	case logrus.ErrorLevel:
		return logrus.Error
	case logrus.WarnLevel:
		return logrus.Warn
	case logrus.InfoLevel:
		return logrus.Info
	case logrus.DebugLevel:
		return logrus.Debug
	default:
		return func(args ...interface{}) {
			fmt.Fprint(os.Stderr, args)
		}
	}
}

func ConfigureLogging(loggingConfig *LoggingConfiguration) error {
	if loggingConfigured {
		logrus.Error("logging has been configured already")
		return nil
	}

	configureDebugLevel(loggingConfig)
	if err := configureLogOutput(loggingConfig); err != nil {
		return err
	}
	if err := configureLogFormat(loggingConfig); err != nil {
		return err
	}

	loggingConfigured = true
	return nil
}

func configureDebugLevel(loggingConfig *LoggingConfiguration) {
	if loggingConfig.IsDebug {
		logrus.SetLevel(logrus.DebugLevel)
	}
}

func configureLogOutput(loggingConfig *LoggingConfiguration) error {
	if loggingConfig.LogFilePath != "" {
		return configureLogFileOutput(loggingConfig.LogFilePath)
	}

	if loggingConfig.LogPipeFd != "" {
		logPipeFdInt, err := strconv.Atoi(loggingConfig.LogPipeFd)
		if err != nil {
			return fmt.Errorf("failed to convert _LIBCONTAINER_LOGPIPE environment variable value %s to int: %v", loggingConfig.LogPipeFd, err)
		}
		return configureLogPipeOutput(logPipeFdInt)
	}

	return nil
}

func configureLogPipeOutput(logPipeFd int) error {
	logrus.SetOutput(os.NewFile(uintptr(logPipeFd), "logpipe"))
	return nil
}

func configureLogFileOutput(logFilePath string) error {
	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	logrus.SetOutput(f)
	return nil
}

func configureLogFormat(loggingConfig *LoggingConfiguration) error {
	switch loggingConfig.LogFormat {
	case "text":
		// retain logrus's default.
	case "json":
		logrus.SetFormatter(new(logrus.JSONFormatter))
	default:
		return fmt.Errorf("unknown log-format %q", loggingConfig.LogFormat)
	}
	return nil
}
