package main

// terminal_ws.go — shared browser terminal endpoint (/ws/terminal).
//
// The Vue dashboard's TerminalWidget connects here; each received
// JSON message {"cmd": "..."} is executed through the interactive
// Console and the captured output (plus the next prompt) is streamed
// back as {"output": "...", "prompt": "..."}.

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/ruby570bocadito/vesper/internal/appstate"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// The dashboard is the only intended client; the endpoint lives
	// behind the same origin as the static dashboard itself.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsTerminalHub serialises access to the single shared console
// ("one Console for all browser tabs", as dashboard_run.go puts it).
var wsTerminalHub struct {
	mu      sync.Mutex
	console *Console
	pr      *io.PipeReader
	pw      *io.PipeWriter
	out     []byte
}

// handleTerminalWS upgrades /ws/terminal to a websocket terminal session.
func handleTerminalWS(state *appstate.AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		ensureSharedConsole(state)
		sendTerminalState(conn) // greet with the initial prompt

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

			// feed the line into the console's stdin pipe
			wsTerminalHub.mu.Lock()
			_, _ = io.WriteString(wsTerminalHub.pw, strings.TrimSpace(req.Cmd)+"\n")
			wsTerminalHub.mu.Unlock()

			// give the console goroutine time to consume and answer
			time.Sleep(150 * time.Millisecond)
			sendTerminalState(conn)
		}
	}
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

	// redirect console output into the hub buffer while the session lives
	orig := ConsoleOut
	ConsoleOut = writerFunc(func(p []byte) (int, error) {
		wsTerminalHub.mu.Lock()
		wsTerminalHub.out = append(wsTerminalHub.out, p...)
		wsTerminalHub.mu.Unlock()
		return len(p), nil
	})

	go func() {
		_ = c.Run() // blocks until the pipe closes or the user quits
		ConsoleOut = orig
	}()
}

// sendTerminalState flushes buffered output + the current prompt.
func sendTerminalState(conn *websocket.Conn) {
	wsTerminalHub.mu.Lock()
	out := string(wsTerminalHub.out)
	wsTerminalHub.out = wsTerminalHub.out[:0]
	c := wsTerminalHub.console
	wsTerminalHub.mu.Unlock()

	if c != nil {
		c.PrintPrompt() // writes into the hub buffer via ConsoleOut
		wsTerminalHub.mu.Lock()
		out += string(wsTerminalHub.out)
		wsTerminalHub.out = wsTerminalHub.out[:0]
		wsTerminalHub.mu.Unlock()
	}

	_ = conn.WriteJSON(map[string]string{
		"output": out,
		"prompt": "",
	})
}

// writerFunc adapts a function to an io.Writer.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }
