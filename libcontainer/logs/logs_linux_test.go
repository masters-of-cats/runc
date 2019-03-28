package logs

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	logFile *os.File
	logR    io.Reader
	logW    io.WriteCloser
)

func TestLoggingToFile(t *testing.T) {
	runLogForwarding(t)
	defer os.Remove(logFile.Name())
	defer logW.Close()

	_, err := logW.Write([]byte(`{"level": "info","msg":"kitten"}`))
	if err != nil {
		t.Fatal(err)
	}

	logFileContent := waitForLogContent(t)
	if !strings.Contains(string(logFileContent), "kitten") {
		t.Fatalf("%s does not contain kitten", string(logFileContent))
	}
}

func TestLogForwardingDoesNotStopOnDecodeErr(t *testing.T) {
	runLogForwarding(t)
	defer os.Remove(logFile.Name())
	defer logW.Close()

	_, err := logW.Write([]byte("kitten\n"))
	if err != nil {
		t.Fatal(err)
	}

	logFileContent := waitForLogContent(t)
	if !strings.Contains(string(logFileContent), "json logs decoding error") {
		t.Fatalf("%q does not contain decoding error", string(logFileContent))
	}

	err = logFile.Truncate(0)
	if err != nil {
		t.Fatal(err)
	}

	_, err = logW.Write([]byte(`{"level": "info","msg":"puppy"}`))
	if err != nil {
		t.Fatal(err)
	}

	logFileContent = waitForLogContent(t)
	if !strings.Contains(string(logFileContent), "puppy") {
		t.Fatalf("%s does not contain puppy", string(logFileContent))
	}
}

func runLogForwarding(t *testing.T) {
	var err error

	logR, logW, err = os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	logFile, err = ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}

	logConfig := &LoggingConfiguration{LogFormat: "json", LogFilePath: logFile.Name()}
	startLogForwarding(t, logConfig)
}

func startLogForwarding(t *testing.T, logConfig *LoggingConfiguration) {
	loggingConfigured = false
	if err := ConfigureLogging(logConfig); err != nil {
		t.Fatal(err)
	}
	go func() {
		ForwardLogs(logR)
	}()
}

func waitForLogContent(t *testing.T) string {
	startTime := time.Now()

	for {
		if time.Now().After(startTime.Add(10 * time.Second)) {
			t.Fatal(errors.New("No content in log file after 10 seconds"))
			break
		}

		fileContent, err := ioutil.ReadAll(logFile)
		if err != nil {
			t.Fatal(err)
		}
		if len(fileContent) == 0 {
			continue
		}
		return string(fileContent)
	}

	return ""
}
