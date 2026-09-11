package main

// terminal_ws.go — shared browser terminal endpoint (/ws/terminal).
//
// The Vue dashboard's TerminalWidget connects here; each received
// JSON message {"cmd": "..."} is executed through the interactive
// Console and the captured output (plus the next prompt) is streamed
// back as {"output": "...", "prompt": "..."}.
//
// Security: the endpoint is same-origin only (browsers sending a
// cross-site Origin are rejected) and, when dashboard.auth_token is
// configured, requires that token via ?token= or an Authorization
// Bearer header. /ws/terminal executes operator commands — it must
// never be reachable by drive-by web pages.

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ruby570bocadito/vesper/internal/appstate"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// Only same-origin browser clients (the dashboard itself) are
	// allowed; non-browser clients send no Origin header and are
	// gated by the auth token instead.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser client (curl, CLI tools)
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return u.Host == r.Host
	},
}

// wsTerminalHub serialises access to the single shared console
// ("one Console for all browser tabs", as dashboard_run.go puts it).
var wsTerminalHub struct {
	mu      sync.Mutex
	console *Console
	pr      *io.PipeReader
	pw      *io.PipeWriter
	out     []byte
	outW    writerFunc // active ConsoleOut redirect (nil when not redirected)
	errW    writerFunc // active ErrOut redirect (nil when not redirected)
}

// handleTerminalWS upgrades /ws/terminal to a websocket terminal session.
// authToken is the configured dashboard.auth_token ("" = no token, the
// endpoint is then still same-origin restricted).
func handleTerminalWS(state *appstate.AppState, authToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" && !terminalTokenOK(r, authToken) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		ensureSharedConsole(state)
		time.Sleep(80 * time.Millisecond) // let Run() print its first prompt
		sendTerminalState(conn, true)

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req struct {
				Cmd string `json:"cmd"`
			}
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}
			if strings.TrimSpace(req.Cmd) == "" {
				continue
			}

			if !feedTerminalLine(strings.TrimSpace(req.Cmd)) {
				// console just exited (or is restarting): recreate it
				ensureSharedConsole(state)
				if !feedTerminalLine(strings.TrimSpace(req.Cmd)) {
					sendTerminalState(conn, false)
					continue
				}
			}

			// drain output until it goes quiet (slow commands like
			// `lab up` produce output for seconds) then answer once
			sendTerminalState(conn, false)
		}
	}
}

// terminalTokenOK checks ?token= or the Authorization Bearer header
// against the configured dashboard token (constant-time).
func terminalTokenOK(r *http.Request, want string) bool {
	got := r.URL.Query().Get("token")
	if got == "" {
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			got = strings.TrimPrefix(h, "Bearer ")
		}
	}
	return got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// feedTerminalLine writes one line into the console stdin pipe.
// Returns false when there is no live console (e.g. it exited).
func feedTerminalLine(line string) bool {
	wsTerminalHub.mu.Lock()
	defer wsTerminalHub.mu.Unlock()
	if wsTerminalHub.pw == nil {
		return false
	}
	// io.Pipe writes block until read; the console goroutine is the
	// reader while it lives, and closing pw on exit turns any late
	// write into an immediate error instead of a deadlock.
	_, err := io.WriteString(wsTerminalHub.pw, line+"\n")
	return err == nil
}

// ensureSharedConsole lazily creates the console backing the WS terminal
// and starts draining it on a background goroutine.
func ensureSharedConsole(state *appstate.AppState) {
	wsTerminalHub.mu.Lock()
	defer wsTerminalHub.mu.Unlock()
	if wsTerminalHub.console != nil {
		return
	}
	pr, pw := io.Pipe()
	wsTerminalHub.pr, wsTerminalHub.pw = pr, pw

	c := NewConsoleWithReader(state, pr)
	c.hideBanner = true
	wsTerminalHub.console = c

	// redirect console output (and errors) into the hub buffer
	origOut, origErr := ConsoleOut, ErrOut
	outW := writerFunc(func(p []byte) (int, error) {
		wsTerminalHub.mu.Lock()
		wsTerminalHub.out = append(wsTerminalHub.out, p...)
		wsTerminalHub.mu.Unlock()
		return len(p), nil
	})
	wsTerminalHub.outW, wsTerminalHub.errW = outW, outW
	ConsoleOut, ErrOut = outW, outW

	go func() {
		_ = c.Run() // blocks until the pipe closes or the user quits

		wsTerminalHub.mu.Lock()
		defer wsTerminalHub.mu.Unlock()
		// only restore if no newer console has taken over the redirect
		if wsTerminalHub.console == c {
			wsTerminalHub.console = nil
			if wsTerminalHub.pw != nil {
				_ = wsTerminalHub.pw.Close() // unblock any pending write
			}
			wsTerminalHub.pr, wsTerminalHub.pw = nil, nil
			// holding the hub mutex: no newer console took
			// over, so the redirect is still ours to undo
			ConsoleOut, ErrOut = origOut, origErr
			wsTerminalHub.outW, wsTerminalHub.errW = nil, nil
		}
	}()
}

// sendTerminalState flushes buffered console output to the websocket in
// chunks until the output goes quiet (400 ms without new bytes) or a 5 s
// deadline hits — slow commands (lab up, bridge calls) stream fully.
// withPrompt prints an explicit prompt first (initial greeting only;
// afterwards the console loop's own prompt is already in the buffer).
func sendTerminalState(conn *websocket.Conn, withPrompt bool) {
	deadline := time.Now().Add(5 * time.Second)
	quiet := 0
	for {
		wsTerminalHub.mu.Lock()
		out := string(wsTerminalHub.out)
		wsTerminalHub.out = wsTerminalHub.out[:0]
		c := wsTerminalHub.console
		wsTerminalHub.mu.Unlock()

		if withPrompt && c != nil && len(out) == 0 {
			c.PrintPrompt() // writes into the hub buffer via ConsoleOut
			wsTerminalHub.mu.Lock()
			out += string(wsTerminalHub.out)
			wsTerminalHub.out = wsTerminalHub.out[:0]
			wsTerminalHub.mu.Unlock()
		}

		if out != "" {
			_ = conn.WriteJSON(map[string]string{"output": out, "prompt": ""})
			quiet = 0
		} else {
			quiet++
			if quiet >= 2 || time.Now().After(deadline) {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// writerFunc adapts a function to an io.Writer.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
