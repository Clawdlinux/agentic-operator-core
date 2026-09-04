package netprobe

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestRunDial(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			_ = connection.Close()
		}
	}()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	var stdout, stderr bytes.Buffer
	if exitCode := Run([]string{"dial", "127.0.0.1", port}, &stdout, &stderr); exitCode != ExitSuccess {
		t.Fatalf("Run(dial) = %d, stderr=%q", exitCode, stderr.String())
	}
	if stdout.String() != "connected\n" {
		t.Fatalf("stdout = %q, want connected", stdout.String())
	}
}

func TestDialReturnsDNSFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if exitCode := dial(ctx, "does-not-exist.invalid", "18443", &bytes.Buffer{}, &bytes.Buffer{}); exitCode != ExitDNSFailed {
		t.Fatalf("dial() = %d, want %d", exitCode, ExitDNSFailed)
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	if exitCode := Run(nil, &bytes.Buffer{}, &bytes.Buffer{}); exitCode != ExitInvalidArgs {
		t.Fatalf("Run() = %d, want %d", exitCode, ExitInvalidArgs)
	}
	if exitCode := Run([]string{"dial", "localhost"}, &bytes.Buffer{}, &bytes.Buffer{}); exitCode != ExitInvalidArgs {
		t.Fatalf("Run(dial) = %d, want %d", exitCode, ExitInvalidArgs)
	}
}
