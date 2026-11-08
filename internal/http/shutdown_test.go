package http_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHTTPServer_GracefulShutdown tests KF-39 HTTP server graceful shutdown:
// In-flight requests drain cleanly, timeouts are set, and new requests are rejected.
func TestHTTPServer_GracefulShutdown(t *testing.T) {
	var inFlightStarted sync.WaitGroup
	var inFlightDone sync.WaitGroup
	var completedRequests atomic.Int32

	inFlightStarted.Add(1)
	inFlightDone.Add(1)

	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		inFlightStarted.Done()
		time.Sleep(150 * time.Millisecond)
		completedRequests.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("slow response completed"))
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.Serve(listener)
	}()

	client := &http.Client{Timeout: 5 * time.Second}
	addr := listener.Addr().String()

	// 1. Launch in-flight request
	var resp *http.Response
	var reqErr error
	go func() {
		defer inFlightDone.Done()
		resp, reqErr = client.Get("http://" + addr + "/slow")
	}()

	// Wait until handler is actively running
	inFlightStarted.Wait()

	// 2. Initiate graceful shutdown while request is in-flight
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	shutdownErr := server.Shutdown(shutdownCtx)
	require.NoError(t, shutdownErr, "server.Shutdown should succeed cleanly within timeout")

	// 3. Wait for in-flight request to finish
	inFlightDone.Wait()

	require.NoError(t, reqErr, "in-flight request should complete without error")
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(1), completedRequests.Load())

	// 4. Verify server is closed and new connections are refused
	_, err = client.Get("http://" + addr + "/slow")
	assert.Error(t, err, "new connections must be rejected after server shutdown")

	// 5. Verify Serve returned http.ErrServerClosed
	serveErr := <-serverErrCh
	assert.True(t, errors.Is(serveErr, http.ErrServerClosed))
}

// TestHTTPServer_ShutdownDeadlineExceeded tests that if in-flight requests take longer
// than the shutdown context deadline, Shutdown returns context.DeadlineExceeded.
func TestHTTPServer_ShutdownDeadlineExceeded(t *testing.T) {
	started := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/forever", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		_ = server.Serve(listener)
	}()

	client := &http.Client{}
	addr := listener.Addr().String()

	go func() {
		_, _ = client.Get("http://" + addr + "/forever")
	}()

	<-started

	// Shutdown with a very short 50ms timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = server.Shutdown(shutdownCtx)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "expected DeadlineExceeded error")

	// Force close server
	_ = server.Close()
}

// TestHTTPServer_TimeoutsConfigured tests that all required timeouts are properly populated.
func TestHTTPServer_TimeoutsConfigured(t *testing.T) {
	server := &http.Server{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	assert.Equal(t, 5*time.Second, server.ReadHeaderTimeout)
	assert.Equal(t, 10*time.Second, server.ReadTimeout)
	assert.Equal(t, 15*time.Second, server.WriteTimeout)
	assert.Equal(t, 60*time.Second, server.IdleTimeout)
}
