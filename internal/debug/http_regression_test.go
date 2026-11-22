package debug

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestApprovedHealthRequestRedactsAndBoundsOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, "token=secret-value trailing data")
	}))
	defer server.Close()
	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	runtime := NewRuntime(Environment{Services: map[string]Service{"api": {Host: host, Port: port, Health: []string{"/health"}}}, Limits: map[string]int{"max_bytes": 18}})
	result, err := runtime.Health("api", "/health")
	if err != nil || result.Status != "ok" || result.HTTPStatus != http.StatusAccepted || strings.Contains(result.Body, "secret") || !strings.Contains(result.Body, "[REDACTED]") || strings.Contains(result.Body, "trailing") {
		t.Fatalf("health = %+v, error = %v", result, err)
	}
	if _, err := runtime.Health("api", "/unapproved"); err == nil {
		t.Fatal("unapproved path accepted")
	}
}

func TestHealthReturnsUnavailableWhenConnectionFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	address := listener.Addr().(*net.TCPAddr)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()
	runtime := NewRuntime(Environment{Services: map[string]Service{"api": {Host: "127.0.0.1", Port: address.Port, Health: []string{"/health"}}}})
	result, err := runtime.Health("api", "/health")
	if err != nil || result.Status != "unavailable" {
		t.Fatalf("health = %+v, error = %v", result, err)
	}
}
