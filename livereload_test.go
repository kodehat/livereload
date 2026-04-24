package livereload_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kodehat/livereload"
)

const (
	contextPath = "/myapp"
	reloadPath  = "/reload"
)

func TestScriptContainsWebSocketPath(t *testing.T) {
	got := string(livereload.Script(livereload.NewParams()))
	if !strings.Contains(got, fmt.Sprintf(`"%s"`, reloadPath)) {
		t.Errorf("Script(\"\") should reference %s endpoint, got: %s", reloadPath, got)
	}
	if !strings.Contains(got, "const maxReconnectCount = 5;") {
		t.Errorf("Script should use the default max reconnect count, got: %s", got)
	}
	if !strings.Contains(got, "const reconnectDelayMs = 1000;") {
		t.Errorf("Script should use the default reconnect delay, got: %s", got)
	}
	if !strings.Contains(got, "<script>") || !strings.Contains(got, "</script>") {
		t.Errorf("Script should be wrapped in <script> tags")
	}
}

func TestScriptWithContextPath(t *testing.T) {
	got := string(livereload.Script(livereload.NewParams(
		livereload.WithContextPath(contextPath),
		livereload.WithReloadPath(reloadPath),
		livereload.WithMaxReconnects(7),
		livereload.WithReconnectDelay(3*time.Second),
	)))
	if !strings.Contains(got, contextPath) {
		t.Errorf("Script(%q) should contain the context path, got: %s", contextPath, got)
	}
	if !strings.Contains(got, reloadPath) {
		t.Errorf("Script should contain the websocket path, got: %s", got)
	}
	if !strings.Contains(got, "const maxReconnectCount = 7;") {
		t.Errorf("Script should contain the custom max reconnect count, got: %s", got)
	}
	if !strings.Contains(got, "const reconnectDelayMs = 3000;") {
		t.Errorf("Script should contain the custom reconnect delay, got: %s", got)
	}
}

func TestHandlerRejectsNonWebSocket(t *testing.T) {
	handler := livereload.Handler(livereload.NewParams())

	req := httptest.NewRequest(http.MethodGet, reloadPath, nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	// A plain HTTP request (no Upgrade header) should not return 200.
	if rec.Code == http.StatusOK {
		t.Errorf("expected non-200 status for plain HTTP request, got %d", rec.Code)
	}
}

func TestHandlerRejectsWrongPath(t *testing.T) {
	handler := livereload.Handler(livereload.NewParams(livereload.WithReloadPath(reloadPath)))

	req := httptest.NewRequest(http.MethodGet, "/wrongpath", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for wrong websocket path, got %d", rec.Code)
	}
}

func TestNewParamsDefaults(t *testing.T) {
	params := livereload.NewParams()

	if params.ReloadPath != reloadPath {
		t.Errorf("expected default reload path %s, got %q", reloadPath, params.ReloadPath)
	}
	if params.PingInterval != 2*time.Second {
		t.Errorf("expected default ping interval 2s, got %s", params.PingInterval)
	}
	if params.MaxReconnects != 5 {
		t.Errorf("expected default max reconnects 5, got %d", params.MaxReconnects)
	}
	if params.ReconnectDelay != time.Second {
		t.Errorf("expected default reconnect delay 1s, got %s", params.ReconnectDelay)
	}
}

func TestNewParamsWithOptions(t *testing.T) {
	params := livereload.NewParams(
		livereload.WithContextPath(contextPath),
		livereload.WithReloadPath(reloadPath),
		livereload.WithPingInterval(3*time.Second),
		livereload.WithMaxReconnects(7),
		livereload.WithReconnectDelay(4*time.Second),
	)

	if params.ContextPath != contextPath || params.ReloadPath != reloadPath {
		t.Fatalf("unexpected path params: %+v", params)
	}
	if params.PingInterval != 3*time.Second || params.MaxReconnects != 7 || params.ReconnectDelay != 4*time.Second {
		t.Fatalf("unexpected timing params: %+v", params)
	}
}
