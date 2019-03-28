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

func ForwardLogs(p io.Reader) {
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
			continue
		}

		lvl, err := logrus.ParseLevel(jl.Level)
		if err != nil {
			logrus.Errorf("failed to parse log level '%s': %v\n", jl.Level, err)
			continue
		}
		log(lvl, jl.Msg)
	}
}

func log(level logrus.Level, args ...interface{}) {
	switch level {
	case logrus.PanicLevel:
		logrus.Panic(args...)
	case logrus.FatalLevel:
		logrus.Fatal(args...)
	case logrus.ErrorLevel:
		logrus.Error(args...)
	case logrus.WarnLevel:
		logrus.Warn(args...)
	case logrus.InfoLevel:
		logrus.Info(args...)
	case logrus.DebugLevel:
		logrus.Debug(args...)
	default:
		logrus.Warnf("Unsupported log level %v while trying to log '%#v'", level, args)
	}
}

func ConfigureLogging(loggingConfig *LoggingConfiguration) error {
	if loggingConfigured {
		logrus.Debug("logging has been already configured")
		return nil
	}

	configureLogLevel(loggingConfig)
	if err := configureLogOutput(loggingConfig); err != nil {
		return err
	}
	if err := configureLogFormat(loggingConfig); err != nil {
		return err
	}

	loggingConfigured = true
	return nil
}

func configureLogLevel(loggingConfig *LoggingConfiguration) {
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
			return fmt.Errorf("failed to convert _LIBCONTAINER_LOGPIPE environment variable value %q to int: %v", loggingConfig.LogPipeFd, err)
		}
		configureLogPipeOutput(logPipeFdInt)
		return nil
	}

	return nil
}

func configureLogPipeOutput(logPipeFd int) {
	logrus.SetOutput(os.NewFile(uintptr(logPipeFd), "logpipe"))
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
