// Package livereload provides a WebSocket-based live-reload handler and a
// JavaScript snippet for use during development. When the server restarts, the
// WebSocket connection drops; the browser-side snippet detects the close,
// reconnects, and calls location.reload() once the server is back up.
//
// Usage:
//
//	// Register the WebSocket endpoint
//	params := livereload.NewParams()
//	mux.HandleFunc(params.ReloadPath, livereload.Handler(params))
//
//	// Inject the JS snippet into every HTML page (before </body> or in <head>)
//	fmt.Fprint(w, livereload.Script(params))
package livereload

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/coder/websocket"
)

const (
	defaultPingInterval   = 2 * time.Second
	defaultMaxReconnects  = 5
	defaultReconnectDelay = 1 * time.Second
	defaultReloadPath     = "/reload"
)

// Params configures the app context path, websocket reload path, and timing
// settings used by the reconnect loop. Use NewParams to construct a Params
// value with defaults and functional options.
type Params struct {
	// ContextPath is the path where the app is mounted.
	ContextPath string

	// ReloadPath is the websocket endpoint path. Empty uses /reload.
	ReloadPath string

	// PingInterval controls how often the server pings the websocket. Zero
	// uses the default 2s interval.
	PingInterval time.Duration

	// MaxReconnects limits how many reconnect attempts the browser makes. Zero
	// uses the default of 5.
	MaxReconnects int

	// ReconnectDelay controls the base delay between reconnect attempts. Zero
	// uses the default of 1s.
	ReconnectDelay time.Duration
}

// Option configures Params values for NewParams.
type Option func(*Params)

// NewParams returns Params with the package defaults applied, then applies any
// provided options.
func NewParams(opts ...Option) Params {
	params := Params{
		ContextPath:    "",
		ReloadPath:     defaultReloadPath,
		PingInterval:   defaultPingInterval,
		MaxReconnects:  defaultMaxReconnects,
		ReconnectDelay: defaultReconnectDelay,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&params)
		}
	}

	return params
}

// WithContextPath sets the app mount path.
func WithContextPath(contextPath string) Option {
	return func(params *Params) {
		params.ContextPath = contextPath
	}
}

// WithReloadPath sets the websocket endpoint path.
func WithReloadPath(reloadPath string) Option {
	return func(params *Params) {
		params.ReloadPath = reloadPath
	}
}

// WithPingInterval sets the websocket ping interval.
func WithPingInterval(pingInterval time.Duration) Option {
	return func(params *Params) {
		params.PingInterval = pingInterval
	}
}

// WithMaxReconnects sets the browser reconnect limit.
func WithMaxReconnects(maxReconnects int) Option {
	return func(params *Params) {
		params.MaxReconnects = maxReconnects
	}
}

// WithReconnectDelay sets the reconnect delay used by the browser script.
func WithReconnectDelay(reconnectDelay time.Duration) Option {
	return func(params *Params) {
		params.ReconnectDelay = reconnectDelay
	}
}

// Handler returns an http.HandlerFunc that upgrades the connection to a
// WebSocket and keeps it alive with periodic pings. Params.PingInterval
// controls the ping cadence. When the server shuts down or restarts, the
// connection is closed with StatusGoingAway, causing the browser-side script
// to detect the disconnect and trigger a reload.
func Handler(params Params) http.HandlerFunc {
	params = normalizeParams(params)
	pingInterval := resolvePingInterval(params.PingInterval)
	reloadPath := resolveReloadPath(params.ReloadPath)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != reloadPath {
			http.NotFound(w, r)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			http.Error(w, "could not open websocket", http.StatusInternalServerError)
			return
		}
		defer conn.Close(websocket.StatusGoingAway, "server closed websocket")

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		socketCtx := conn.CloseRead(ctx)
		for {
			select {
			case <-socketCtx.Done():
				return
			case <-time.After(pingInterval):
				if err := conn.Ping(socketCtx); err != nil {
					return
				}
			}
		}
	}
}

// Script returns the JavaScript snippet that should be injected into every
// HTML page during development. It opens a WebSocket to params.ContextPath+
// params.ReloadPath, uses Params.MaxReconnects and Params.ReconnectDelay for
// reconnect behavior, and calls location.reload() when the server comes back
// after a restart.
//
// The returned value is html/template.HTML so it can be used safely with
// Go's html/template package without additional escaping.
func Script(params Params) template.HTML {
	params = normalizeParams(params)
	contextPath := params.ContextPath
	reloadPath := resolveReloadPath(params.ReloadPath)
	maxReconnects := resolveMaxReconnects(params.MaxReconnects)
	reconnectDelay := resolveReconnectDelay(params.ReconnectDelay)
	return template.HTML(fmt.Sprintf(`<script>
	(function() {
		const contextPath = %q;
		const reloadPath = %q;
		const maxReconnectCount = %d;
		const reconnectDelayMs = %d;

		var reconnectCount = 0;
		var isReloading = false;

		function connect() {
			console.log("[🤖] Connecting to WebSocket.");
			var ws = new WebSocket("ws://" + document.location.host + contextPath + reloadPath);

			ws.onopen = function() {
				if (reconnectCount > 0) {
					console.log("[🤖] Socket reconnected. Forcing browser reload.");
					isReloading = true;
					ws.close();
					location.reload();
				} else {
					console.log("[🤖] Socket connected.");
				}
			};

			ws.onclose = function(e) {
				if (isReloading) {
					return;
				}
				if (reconnectCount >= maxReconnectCount) {
					console.error("[🤖] Maximum reconnect count reached. Please refresh the page manually.");
					ws.close();
					return;
				}
				var reconnectRetry = reconnectCount++ * reconnectDelayMs + reconnectDelayMs;
				console.log("[🤖] Socket was closed. Reconnect #" + reconnectCount + " will be attempted in " + reconnectRetry + " ms.");
				setTimeout(connect, reconnectRetry);
			};
		}

		// Prevent reconnect logic from triggering on manual page refresh.
		window.addEventListener('beforeunload', function() {
			isReloading = true;
		});

		connect();
	})();
</script>`, contextPath, reloadPath, maxReconnects, reconnectDelay.Milliseconds()))
}

func resolvePingInterval(pingInterval time.Duration) time.Duration {
	if pingInterval <= 0 {
		return defaultPingInterval
	}

	return pingInterval
}

func resolveMaxReconnects(maxReconnects int) int {
	if maxReconnects <= 0 {
		return defaultMaxReconnects
	}

	return maxReconnects
}

func resolveReconnectDelay(reconnectDelay time.Duration) time.Duration {
	if reconnectDelay <= 0 {
		return defaultReconnectDelay
	}

	return reconnectDelay
}

func resolveReloadPath(reloadPath string) string {
	if reloadPath == "" {
		return defaultReloadPath
	}

	return reloadPath
}

func normalizeParams(params Params) Params {
	defaults := NewParams()

	if params.ReloadPath == "" {
		params.ReloadPath = defaults.ReloadPath
	}
	if params.PingInterval <= 0 {
		params.PingInterval = defaults.PingInterval
	}
	if params.MaxReconnects <= 0 {
		params.MaxReconnects = defaults.MaxReconnects
	}
	if params.ReconnectDelay <= 0 {
		params.ReconnectDelay = defaults.ReconnectDelay
	}

	return params
}
