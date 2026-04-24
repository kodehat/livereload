# livereload

`livereload` is a small Go helper for development-time live reloading. It exposes a WebSocket endpoint and provides a browser snippet that reconnects after a server restart and refreshes the page automatically.

## Installation

```bash
go get github.com/kodehat/livereload
```

## Requirements

- Go 1.26+
- A `net/http` server

## Usage

Register the WebSocket endpoint:

> Only register the route and inject the script when your app is running in a development mode.

```go
package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/kodehat/livereload"
)

func main() {
	mux := http.NewServeMux()

	params := livereload.NewParams()

	mux.HandleFunc(params.ReloadPath, livereload.Handler(params))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`
<!doctype html>
<html>
  <head>
    <title>Example</title>
  </head>
  <body>
    <h1>Hello</h1>
    ` + string(livereload.Script(params)) + `
  </body>
</html>`))
	})

	http.ListenAndServe(":8080", mux)
}
```

If your app is mounted under a sub-path, build params with options:

```go
params := livereload.NewParams(
	livereload.WithContextPath("/myapp"),
	livereload.WithReloadPath("/events"),
	livereload.WithPingInterval(5*time.Second),
	livereload.WithMaxReconnects(10),
	livereload.WithReconnectDelay(2*time.Second),
)

fmt.Fprint(w, livereload.Script(params))
```

## API

### `Params`

`Params` stores the app `ContextPath`, websocket `ReloadPath`, and reconnect timing.

### `NewParams(opts ...Option) Params`

Creates a `Params` value with these defaults:

- `ContextPath`: `""` (root-mounted app)
- `ReloadPath`: `"/reload"`
- `PingInterval`: `2s`
- `MaxReconnects`: `5`
- `ReconnectDelay`: `1s`

Use `WithContextPath`, `WithReloadPath`, `WithPingInterval`, `WithMaxReconnects`, and `WithReconnectDelay` to override specific values.

### `Option`

Functional option used by `NewParams`.

### `WithContextPath`, `WithReloadPath`, `WithPingInterval`, `WithMaxReconnects`, `WithReconnectDelay`

Helpers for customizing `Params` when constructing it with `NewParams`.

### `Handler(params Params) http.HandlerFunc`

Returns a handler for the websocket endpoint. `params.PingInterval` controls how often the connection is pinged.

### `Script(params Params) template.HTML`

Returns the JavaScript snippet to inject into HTML pages. `params.ContextPath` is the app base path, `params.ReloadPath` overrides the websocket endpoint path, and `params.MaxReconnects` plus `params.ReconnectDelay` control the reconnect loop.

## How it works

1. The browser opens a WebSocket connection to `contextPath + reloadPath`.
2. While the server is running, the connection stays alive.
3. When the server restarts, the socket closes.
4. The script retries the connection.
5. Once the connection is restored, the page reloads automatically.

## Notes

- The snippet is intended for development use.
- Keep both the route and the script out of production builds.
- Inject it into your HTML before `</body>` or in `<head>`.
