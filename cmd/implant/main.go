// Vesper implant — C2 beacon binary (formerly cmd/implant in Vesper).
//
// Payload modes:
//
//	beacon (default): periodic reachability beacon with jittered backoff.
//
// Safety: this binary is a lab/teaching artifact. Destructive payload
// modes were removed in the Vesper remodel; the agent's post-exploit
// modules are gated by Simulation=true defaults and the CLI auth gate.
package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

var (
	C2Host      = "localhost"
	C2Port      = "8443"
	PayloadType = "beacon"
	Stealth     = "false"
)

const (
	beaconBaseInterval = 30 * time.Second
	beaconJitterMax    = 15 * time.Second
	maxBackoff         = 5 * time.Minute
	reconnectBase      = 2 * time.Second
)

func main() {
	if Stealth == "true" {
		jitterSleep(3*time.Second, 5*time.Second)
	}

	c2Addr := fmt.Sprintf("%s:%s", C2Host, C2Port)

	switch PayloadType {
	case "beacon", "":
		runBeacon(c2Addr)
	default:
		fmt.Fprintf(os.Stderr, "unknown payload type %q (supported: beacon)\n", PayloadType)
		os.Exit(2)
	}
}

func runBeacon(c2 string) {
	backoff := reconnectBase
	for {
		if isC2Reachable(c2) {
			backoff = reconnectBase
			jitterSleep(beaconBaseInterval, beaconJitterMax)
		} else {
			jitterSleep(backoff, backoff/3)
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func isC2Reachable(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func jitterSleep(base, jitter time.Duration) {
	extra := time.Duration(rand.Int63n(int64(jitter)))
	time.Sleep(base + extra)
}
