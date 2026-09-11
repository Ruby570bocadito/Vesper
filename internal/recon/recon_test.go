package recon

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestScanHostDetectsOpenPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Write([]byte("TEST-BANNER\n"))
			conn.Close()
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	res := ScanHost(context.Background(), "127.0.0.1", []int{port}, 500*time.Millisecond, true)

	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if !res[0].Open {
		t.Fatalf("port %d should be open", port)
	}
	if res[0].Banner == "" || !contains(res[0].Banner, "TEST-BANNER") {
		t.Fatalf("expected banner, got %q", res[0].Banner)
	}
}

func TestScanHostClosedPort(t *testing.T) {
	// Port 9 is almost never open locally (discard); if it somehow is,
	// pick another unlikely one. We assert NOT-open on a listener we
	// never start: bind one to RESERVE the port, close it, then scan.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	time.Sleep(50 * time.Millisecond)

	res := ScanHost(context.Background(), "127.0.0.1", []int{port}, 500*time.Millisecond, true)
	if res[0].Open {
		t.Fatalf("port %d should be closed", port)
	}
}

func TestScanHostHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res := ScanHost(ctx, "127.0.0.1", DefaultPorts, 10*time.Millisecond, false)
	// With a cancelled context the scan returns without blocking.
	// Results may or may not be populated — just don't hang/panic.
	_ = res
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestExpandTargetsCIDR(t *testing.T) {
	got, err := ExpandTargets("192.168.10.0/30")
	if err != nil {
		t.Fatalf("ExpandTargets: %v", err)
	}
	want := []string{"192.168.10.0", "192.168.10.1", "192.168.10.2", "192.168.10.3"}
	if len(got) != len(want) {
		t.Fatalf("got %d hosts, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("host[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestExpandTargetsSingleIPAndList(t *testing.T) {
	got, err := ExpandTargets("127.0.0.1, 127.0.0.2")
	if err != nil {
		t.Fatalf("ExpandTargets: %v", err)
	}
	if len(got) != 2 || got[0] != "127.0.0.1" || got[1] != "127.0.0.2" {
		t.Fatalf("unexpected expansion: %v", got)
	}
}

func TestExpandTargetsRejectsInvalid(t *testing.T) {
	if _, err := ExpandTargets(""); err == nil {
		t.Fatal("empty target should error")
	}
	if _, err := ExpandTargets("not-a-host-or-cidr"); err == nil {
		t.Fatal("garbage target should error")
	}
	if _, err := ExpandTargets("10.0.0.0/64"); err == nil {
		t.Fatal("invalid CIDR should error")
	}
}

func TestExpandTargetsCapsSweep(t *testing.T) {
	// /16 would be 65k hosts — must cap at MaxScanHosts
	got, err := ExpandTargets("10.1.0.0/16")
	if err != nil {
		t.Fatalf("ExpandTargets: %v", err)
	}
	if len(got) != MaxScanHosts {
		t.Fatalf("cap: got %d hosts, want %d", len(got), MaxScanHosts)
	}
}

func TestScanTargetsSweepsMultipleHosts(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port

	res := ScanTargets(context.Background(), []string{"127.0.0.1"},
		[]int{port}, 500*time.Millisecond, false)
	if len(res) != 1 || len(res["127.0.0.1"]) != 1 || !res["127.0.0.1"][0].Open {
		t.Fatalf("ScanTargets result: %#v", res)
	}
}
