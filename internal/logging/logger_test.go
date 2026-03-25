package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetupCreatesFileAndWritesLogs(t *testing.T) {
	oldOut := log.Writer()
	oldGinOut := gin.DefaultWriter
	oldGinErr := gin.DefaultErrorWriter
	defer func() {
		log.SetOutput(oldOut)
		gin.DefaultWriter = oldGinOut
		gin.DefaultErrorWriter = oldGinErr
	}()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "nested", "opensbx.log")

	closer, err := Setup(logPath)
	if err != nil {
		t.Fatalf("Setup() error: %v", err)
	}
	defer closer.Close()

	log.Print("logger-test-line")
	if _, err := gin.DefaultWriter.Write([]byte("gin-log-line\n")); err != nil {
		t.Fatalf("write gin log: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "logger-test-line") {
		t.Fatalf("log file missing stdlib log line: %q", content)
	}
	if !strings.Contains(content, "gin-log-line") {
		t.Fatalf("log file missing gin log line: %q", content)
	}
}

func TestSetupInvalidPath(t *testing.T) {
	if _, err := Setup(""); err == nil {
		t.Fatalf("Setup(\"\") should return error")
	}
}
