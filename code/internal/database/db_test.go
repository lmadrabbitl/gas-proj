package database

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestNewExitsOnInvalidDSN(t *testing.T) {
	t.Parallel()

	if os.Getenv("GO_WANT_DATABASE_HELPER_PROCESS") == "1" {
		New()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestNewExitsOnInvalidDSN$")
	cmd.Env = append(os.Environ(),
		"GO_WANT_DATABASE_HELPER_PROCESS=1",
		"DATABASE_URL=://bad-dsn",
	)

	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected New to exit the process on invalid DSN")
	}
	if !strings.Contains(string(output), "Error when opening DB connection") {
		t.Fatalf("expected fatal connection message, got %q", string(output))
	}
}
