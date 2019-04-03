package logs

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	logFile string
	logR    *os.File
	logW    *os.File
)

func TestLoggingToFile(t *testing.T) {
	runLogForwarding(t)
	defer os.Remove(logFile)
	defer logW.Close()

	logToLogWriter(t, `{"level": "info","msg":"kitten"}`)

	logFileContent := waitForLogContent(t)
	if !strings.Contains(string(logFileContent), "kitten") {
		t.Fatalf("%s does not contain kitten", string(logFileContent))
	}
}

func TestLogForwardingDoesNotStopOnJsonDecodeErr(t *testing.T) {
	runLogForwarding(t)
	defer os.Remove(logFile)
	defer logW.Close()

	logToLogWriter(t, "invalid-json-with-kitten")

	logFileContent := waitForLogContent(t)
	if !strings.Contains(string(logFileContent), "failed to decode") {
		t.Fatalf("%q does not contain decoding error", string(logFileContent))
	}

	truncateLogFile(t)

	logToLogWriter(t, `{"level": "info","msg":"puppy"}`)

	logFileContent = waitForLogContent(t)
	if !strings.Contains(string(logFileContent), "puppy") {
		t.Fatalf("%s does not contain puppy", string(logFileContent))
	}
}

func TestLogForwardingDoesNotStopOnLogLevelParsingErr(t *testing.T) {
	runLogForwarding(t)
	defer os.Remove(logFile)
	defer logW.Close()

	logToLogWriter(t, `{"level": "alert","msg":"puppy"}`)

	logFileContent := waitForLogContent(t)
	if !strings.Contains(string(logFileContent), "failed to parse log level") {
		t.Fatalf("%q does not contain log level parsing error", string(logFileContent))
	}

	truncateLogFile(t)

	logToLogWriter(t, `{"level": "info","msg":"puppy"}`)

	logFileContent = waitForLogContent(t)
	if !strings.Contains(string(logFileContent), "puppy") {
		t.Fatalf("%s does not contain puppy", string(logFileContent))
	}
}

func logToLogWriter(t *testing.T, message string) {
	_, err := logW.Write([]byte(message + "\n"))
	if err != nil {
		t.Fatalf("failed to write %q to log writer: %v", message, err)
	}
}

func runLogForwarding(t *testing.T) {
	var err error

	logR, logW, err = os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	tempFile, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	logFile = tempFile.Name()

	logConfig := &LoggingConfiguration{LogFormat: "json", LogFilePath: logFile}
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

		fileContent, err := ioutil.ReadFile(logFile)
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

func truncateLogFile(t *testing.T) {
	file, err := os.OpenFile(logFile, os.O_RDWR, 0666)
	if err != nil {
		t.Fatalf("failed to open log file: %v", err)
		return
	}
	defer file.Close()

	err = file.Truncate(0)
	if err != nil {
		t.Fatalf("failed to truncate log file: %v", err)
	}
}
