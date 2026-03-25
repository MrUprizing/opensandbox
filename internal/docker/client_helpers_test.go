package docker

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/network"
	"opensbx/internal/database"
)

func TestNormalizePort(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"3000", "3000/tcp"},
		{"3000/tcp", "3000/tcp"},
	}

	for _, tt := range tests {
		if got := normalizePort(tt.in); got != tt.want {
			t.Fatalf("normalizePort(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestNormalizePorts(t *testing.T) {
	got := normalizePorts([]string{"3000", "", "8080/udp"})
	want := []string{"3000/tcp", "8080/udp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizePorts() = %v, want %v", got, want)
	}
}

func TestBuildExposedPorts(t *testing.T) {
	if ps := buildExposedPorts(nil); ps != nil {
		t.Fatalf("buildExposedPorts(nil) should be nil")
	}

	ps := buildExposedPorts([]string{"3000/tcp", "bad"})
	if ps == nil {
		t.Fatalf("buildExposedPorts() should not be nil")
	}
	p3000, err := network.ParsePort("3000/tcp")
	if err != nil {
		t.Fatalf("ParsePort error: %v", err)
	}
	if _, ok := ps[p3000]; !ok {
		t.Fatalf("expected 3000/tcp in exposed ports: %v", ps)
	}
}

func TestBuildPortBindings(t *testing.T) {
	if pm := buildPortBindings(nil); pm != nil {
		t.Fatalf("buildPortBindings(nil) should be nil")
	}

	pm := buildPortBindings([]string{"3000/tcp", "bad"})
	if pm == nil {
		t.Fatalf("buildPortBindings() should not be nil")
	}
	p3000, err := network.ParsePort("3000/tcp")
	if err != nil {
		t.Fatalf("ParsePort error: %v", err)
	}
	b := pm[p3000]
	if len(b) != 1 {
		t.Fatalf("expected one binding, got %d", len(b))
	}
	if b[0].HostIP.String() != "127.0.0.1" {
		t.Fatalf("HostIP = %s, want 127.0.0.1", b[0].HostIP.String())
	}
}

func TestExtractPortsAndPortKeys(t *testing.T) {
	pm := buildPortBindings([]string{"3000/tcp", "8080/tcp"})
	ports := extractPorts(pm)

	if _, ok := ports["3000/tcp"]; !ok {
		t.Fatalf("missing 3000/tcp in extracted ports: %v", ports)
	}
	if _, ok := ports["8080/tcp"]; !ok {
		t.Fatalf("missing 8080/tcp in extracted ports: %v", ports)
	}

	keys := portKeys(ports)
	want := []string{"3000/tcp", "8080/tcp"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("portKeys() = %v, want %v", keys, want)
	}
}

func TestContainerName(t *testing.T) {
	if got := containerName(nil); got != "" {
		t.Fatalf("containerName(nil) = %q, want empty", got)
	}
	if got := containerName([]string{"/demo"}); got != "demo" {
		t.Fatalf("containerName() = %q, want demo", got)
	}
}

func TestPortHelpers(t *testing.T) {
	if got := portValue(3000); got != "3000" {
		t.Fatalf("portValue(3000) = %q, want 3000", got)
	}
	if got := portKey(3000, ""); got != "3000/tcp" {
		t.Fatalf("portKey default proto = %q, want 3000/tcp", got)
	}
	if got := portKey(53, "udp"); got != "53/udp" {
		t.Fatalf("portKey udp = %q, want 53/udp", got)
	}
}

func TestWrapNotFound(t *testing.T) {
	if got := wrapNotFound(nil); got != nil {
		t.Fatalf("wrapNotFound(nil) = %v, want nil", got)
	}

	if got := wrapNotFound(errdefs.ErrNotFound); !errors.Is(got, ErrNotFound) {
		t.Fatalf("wrapNotFound(not found) = %v, want ErrNotFound", got)
	}

	original := errors.New("boom")
	if got := wrapNotFound(original); !errors.Is(got, original) {
		t.Fatalf("wrapNotFound(other) = %v, want original", got)
	}
}

func TestGenerateCmdID(t *testing.T) {
	id := generateCmdID()
	if !strings.HasPrefix(id, "cmd_") {
		t.Fatalf("generateCmdID prefix = %q", id)
	}
	if len(id) != 44 {
		t.Fatalf("generateCmdID length = %d, want 44", len(id))
	}
}

func TestTimerHelpers(t *testing.T) {
	c := &Client{}
	c.scheduleStop("sb-1", 10)

	entry := c.getTimerEntry("sb-1")
	if entry == nil {
		t.Fatalf("expected timer entry")
	}

	c.cancelTimer("sb-1")
	time.Sleep(10 * time.Millisecond)

	if c.getTimerEntry("sb-1") != nil {
		t.Fatalf("expected timer entry to be removed after cancel")
	}
}

func TestDBCommandToDetail(t *testing.T) {
	c := &Client{}
	exitCode := 0
	finishedAt := int64(999)
	detail := c.dbCommandToDetail(database.Command{
		ID:         "cmd-1",
		SandboxID:  "sb-1",
		Name:       "echo",
		Args:       `["hello"]`,
		Cwd:        "/app",
		ExitCode:   &exitCode,
		StartedAt:  100,
		FinishedAt: &finishedAt,
	})

	if detail.ID != "cmd-1" || detail.Name != "echo" || detail.SandboxID != "sb-1" {
		t.Fatalf("detail mismatch: %+v", detail)
	}
	if len(detail.Args) != 1 || detail.Args[0] != "hello" {
		t.Fatalf("args mismatch: %+v", detail.Args)
	}
	if detail.ExitCode == nil || *detail.ExitCode != 0 {
		t.Fatalf("exit code mismatch: %+v", detail.ExitCode)
	}
}
