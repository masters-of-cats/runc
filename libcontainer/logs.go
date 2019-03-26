package libcontainer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

func forwardLogs(p *os.File) {
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
