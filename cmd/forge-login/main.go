// forge-login obtains a local Keycloak access token using browser login and PKCE.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/octieght18/forge/internal/login"
)

func main() { os.Exit(run()) }
func run() int {
	file := flag.String("token-file", "", "private token output file (required)")
	flag.Parse()
	if *file == "" {
		fmt.Fprintln(os.Stderr, "--token-file is required")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	h, err := login.New(ctx, "http://127.0.0.1:8082/realms/forge", "http://127.0.0.1:8083/callback", *file)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Login initialization failed")
		return 1
	}
	l, err := net.Listen("tcp", "127.0.0.1:8083")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Login port unavailable")
		return 1
	}
	s := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 * 1024}
	result := make(chan error, 1)
	go func() { result <- s.Serve(l) }()
	fmt.Println("Open http://127.0.0.1:8083/login in your browser. Login expires in five minutes.")
	select {
	case <-ctx.Done():
	case <-h.Done():
	case <-result:
	}
	drain, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	_ = s.Shutdown(drain)
	// Completion message is in the browser; credentials and tokens are never printed.
	if ctx.Err() != nil || !h.Succeeded() {
		return 1
	}
	return 0
}
