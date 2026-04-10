package cmd

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/kagento/kagento-cli/internal/client"
)

func TestRunRegistryLoginPrintsCommand(t *testing.T) {
	sessionID := "11111111-1111-1111-1111-111111111111"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/registry/token" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer cli-token" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"username":"session-user","password":"token-123","registry":"registry.kagento.io","expires_in":900}`)
	}))
	defer server.Close()

	oldClient := cl
	oldTool := registryTool
	oldPrintCommand := registryPrintCommand
	oldPrintToken := registryPrintToken
	oldStdout := os.Stdout
	defer func() {
		cl = oldClient
		registryTool = oldTool
		registryPrintCommand = oldPrintCommand
		registryPrintToken = oldPrintToken
		os.Stdout = oldStdout
	}()

	cl = &client.Client{
		BackendURL:  server.URL,
		Token:       "cli-token",
		StaticToken: true,
	}
	registryTool = "docker"
	registryPrintCommand = true
	registryPrintToken = false

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = writer

	runRegistryLogin(nil, []string{sessionID})

	_ = writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	text := string(output)
	if !strings.Contains(text, "docker login registry.kagento.io -u 'session-user' --password-stdin") {
		t.Fatalf("output missing login command: %s", text)
	}
	if !strings.Contains(text, "registry.kagento.io/sessions/"+sessionID+"/app:dev") {
		t.Fatalf("output missing example push target: %s", text)
	}
}
