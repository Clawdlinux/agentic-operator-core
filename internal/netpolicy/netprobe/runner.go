package netprobe

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

const (
	ExitSuccess         = 0
	ExitDialFailed      = 10
	ExitDNSFailed       = 20
	ExitInvalidArgs     = 30
	ExitServerFailed    = 40
	defaultDialTimeout  = 5 * time.Second
	defaultServeTimeout = 2 * time.Minute
)

// Run executes the minimal TCP server or client used by the active policy probe.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprintln(stderr, "usage: netprobe serve|dial")
		return ExitInvalidArgs
	}
	switch args[0] {
	case "serve":
		return runServeCommand(args[1:], stderr)
	case "dial":
		return runDialCommand(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown netprobe command %q\n", args[0])
		return ExitInvalidArgs
	}
}

func runServeCommand(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	address := flags.String("address", ":18443", "TCP listen address")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return ExitInvalidArgs
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultServeTimeout)
	defer cancel()
	if err := serve(ctx, *address); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return ExitServerFailed
	}
	return ExitSuccess
}

func serve(ctx context.Context, address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer listener.Close()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("accept: %w", err)
		}
		_ = connection.Close()
	}
}

func runDialCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		_, _ = fmt.Fprintln(stderr, "usage: netprobe dial <host> <port>")
		return ExitInvalidArgs
	}
	if _, err := strconv.ParseUint(args[1], 10, 16); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid TCP port %q\n", args[1])
		return ExitInvalidArgs
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	defer cancel()
	return dial(ctx, args[0], args[1], stdout, stderr)
}

func dial(ctx context.Context, host, port string, stdout, stderr io.Writer) int {
	if _, err := net.DefaultResolver.LookupHost(ctx, host); err != nil {
		_, _ = fmt.Fprintf(stderr, "resolve %s: %v\n", host, err)
		return ExitDNSFailed
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "dial %s:%s: %v\n", host, port, err)
		return ExitDialFailed
	}
	defer connection.Close()
	_, _ = fmt.Fprintln(stdout, "connected")
	return ExitSuccess
}
