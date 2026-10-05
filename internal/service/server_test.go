package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/config"
	"github.com/octieght18/forge/internal/httpapi"
)

type runningServer struct {
	url    string
	stop   context.CancelFunc
	done   chan error
	ready  *httpapi.Readiness
	client *http.Client
}

func startServer(t *testing.T, handler http.Handler, configure func(*config.Config)) *runningServer {
	t.Helper()
	c, err := config.Load(func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	c.ShutdownTimeout = time.Second
	if configure != nil {
		configure(&c)
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ready := &httpapi.Readiness{}
	if handler == nil {
		handler = httpapi.NewHandler(logger, ready)
	}
	s, err := New(c, logger, ready, handler)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	r := &runningServer{"http://" + listener.Addr().String(), stop, make(chan error, 1), ready, &http.Client{Timeout: 3 * time.Second}}
	go func() { r.done <- s.Run(ctx, listener) }()
	t.Cleanup(func() { stop(); r.client.CloseIdleConnections() })
	return r
}

func await[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for test synchronization")
	}
	var zero T
	return zero
}

func TestHealthOverRealHTTP(t *testing.T) {
	r := startServer(t, nil, nil)
	for _, method := range []string{"GET", "HEAD"} {
		request, _ := http.NewRequest(method, r.url+"/readyz", nil)
		response, err := r.client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || !r.ready.Ready() {
			t.Fatalf("bad readiness response: %v %d", err, response.StatusCode)
		}
		if method == "HEAD" && len(body) != 0 {
			t.Fatal("HEAD response included body")
		}
	}
	r.stop()
	if err := await(t, r.done); err != nil {
		t.Fatal(err)
	}
	if r.ready.Ready() {
		t.Fatal("stopped service remains ready")
	}
}

func TestGracefulShutdownDrainsInFlightRequest(t *testing.T) {
	entered, release, canceled := make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)
	r := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(entered)
		select {
		case <-release:
			fmt.Fprint(w, "completed")
		case <-req.Context().Done():
			canceled <- struct{}{}
		}
	}), nil)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	result := make(chan error, 1)
	go func() {
		response, err := r.client.Get(r.url)
		if err == nil {
			var body []byte
			body, err = io.ReadAll(response.Body)
			response.Body.Close()
			if err == nil && string(body) != "completed" {
				err = fmt.Errorf("response did not complete: %q", body)
			}
		}
		result <- err
	}()
	await(t, entered)
	r.stop()
	deadline := time.Now().Add(time.Second)
	for r.ready.Ready() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if r.ready.Ready() {
		t.Fatal("draining service remains ready")
	}
	select {
	case <-canceled:
		t.Fatal("request canceled before grace elapsed")
	default:
	}
	close(release)
	if err := await(t, result); err != nil {
		t.Fatal(err)
	}
	if err := await(t, r.done); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownDeadlineCancelsRequest(t *testing.T) {
	entered, canceled := make(chan struct{}), make(chan struct{})
	r := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(entered)
		<-req.Context().Done()
		close(canceled)
	}), func(c *config.Config) { c.ShutdownTimeout = 30 * time.Millisecond })
	result := make(chan error, 1)
	go func() {
		response, err := r.client.Get(r.url)
		if response != nil {
			response.Body.Close()
		}
		result <- err
	}()
	await(t, entered)
	r.stop()
	if err := await(t, r.done); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want shutdown deadline, got %v", err)
	}
	await(t, canceled)
	await(t, result)
}

func TestClientDisconnectCancelsRequest(t *testing.T) {
	entered, canceled := make(chan struct{}), make(chan struct{})
	r := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(entered)
		<-req.Context().Done()
		close(canceled)
	}), nil)
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(r.url, "http://"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprint(conn, "GET / HTTP/1.1\r\nHost: localhost\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	await(t, entered)
	conn.Close()
	await(t, canceled)
	r.stop()
	if err := await(t, r.done); err != nil {
		t.Fatal(err)
	}
}

func TestSlowHeadersAreClosed(t *testing.T) {
	r := startServer(t, nil, func(c *config.Config) { c.ReadHeaderTimeout = 30 * time.Millisecond })
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(r.url, "http://"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	fmt.Fprint(conn, "GET /healthz HTTP/1.1\r\nHost: localhost\r\nX-Slow: ")
	_, err = conn.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("slow incomplete headers were accepted")
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		t.Fatal("server did not enforce header timeout")
	}
	r.stop()
	if err := await(t, r.done); err != nil {
		t.Fatal(err)
	}
}

func TestUnexpectedListenerFailureIsReported(t *testing.T) {
	c, _ := config.Load(func(string) (string, bool) { return "", false })
	ready := &httpapi.Readiness{}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	s, _ := New(c, logger, ready, httpapi.NewHandler(logger, ready))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
	if err := s.Run(context.Background(), listener); err == nil {
		t.Fatal("closed listener accepted")
	}
	if ready.Ready() {
		t.Fatal("failed service remains ready")
	}
}

func TestSlowBodyReadTimesOut(t *testing.T) {
	readResult := make(chan error, 1)
	r := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, err := io.ReadAll(req.Body)
		readResult <- err
		w.WriteHeader(http.StatusRequestTimeout)
	}), func(c *config.Config) {
		c.ReadHeaderTimeout = 20 * time.Millisecond
		c.ReadTimeout = 80 * time.Millisecond
	})
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(r.url, "http://"), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprint(conn, "POST / HTTP/1.1\r\nHost: localhost\r\nContent-Length: 100\r\n\r\nx")
	err = await(t, readResult)
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected body read timeout, got %v", err)
	}
	r.stop()
	if err := await(t, r.done); err != nil {
		t.Fatal(err)
	}
}

func TestResponseWriteDeadlineIsEnforced(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	writeResult := make(chan error, 1)
	r := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		close(entered)
		<-release
		_, err := w.Write(make([]byte, 64*1024)) // exceed the response buffer to reach the socket
		writeResult <- err
	}), func(c *config.Config) { c.WriteTimeout = 30 * time.Millisecond })
	result := make(chan error, 1)
	go func() {
		response, err := r.client.Get(r.url)
		if response != nil {
			response.Body.Close()
		}
		result <- err
	}()
	await(t, entered)
	time.Sleep(80 * time.Millisecond) // allow the transport deadline to pass, not a throughput assertion
	close(release)
	err := await(t, writeResult)
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected response write timeout, got %v", err)
	}
	await(t, result)
	r.stop()
	if err := await(t, r.done); err != nil {
		t.Fatal(err)
	}
}
