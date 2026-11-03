package frontend

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/0xdevelop/project_template_go/frontend/frontend_config"
)

func TestFrontendDisabledAndOccupiedPort(t *testing.T) {
	for _, cfg := range []*frontend_config.FrontendConfig{nil, {Enabled: false}} {
		server, err := FrontendServicesStart(cfg)
		if err != nil || server != nil {
			t.Fatalf("disabled frontend: server=%v err=%v", server, err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	server, err := FrontendServicesStart(&frontend_config.FrontendConfig{
		Enabled: true, BindAddress: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port,
	})
	if server != nil {
		_ = server.Close()
	}
	if err == nil {
		t.Fatal("occupied port must fail instead of selecting another port")
	}
}

func TestFrontendHTTPAndShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	server, err := FrontendServicesStart(&frontend_config.FrontendConfig{
		Enabled: true, BindAddress: "127.0.0.1", Port: port,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	client := &http.Client{Timeout: 5 * time.Second}
	for _, testCase := range []struct {
		method, path, contentType, body string
		empty                           bool
	}{
		{method: "GET", path: "/", contentType: "text/html", body: "frontend-example"},
		{method: "GET", path: "/index.js", contentType: "javascript", body: "this is example frontend"},
		{method: "GET", path: "/index.css", contentType: "text/css", body: "font-family"},
		{method: "HEAD", path: "/", contentType: "text/html", empty: true},
		{method: "GET", path: "/src/index.ts", empty: true},
		{method: "GET", path: "/package.json", empty: true},
		{method: "GET", path: "/scripts/", empty: true},
		{method: "GET", path: "/missing.js", empty: true},
		{method: "POST", path: "/", empty: true},
	} {
		t.Run(testCase.method+testCase.path, func(t *testing.T) {
			request, err := http.NewRequest(testCase.method, "http://"+server.Addr+testCase.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK || response.Request.URL.Path != testCase.path {
				t.Fatalf("unexpected status or redirect: %s %s", response.Status, response.Request.URL.Path)
			}
			if testCase.empty && len(body) != 0 {
				t.Fatalf("expected neutral empty response, got %q", body)
			}
			if !strings.Contains(response.Header.Get("Content-Type"), testCase.contentType) ||
				!strings.Contains(string(body), testCase.body) {
				t.Fatalf("unexpected content type or body: %s %q", response.Header.Get("Content-Type"), body)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if err == nil {
		_ = connection.Close()
		t.Fatal("frontend listener remains open after shutdown")
	}
}
