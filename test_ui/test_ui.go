// Package test_ui provides the development-only API test console.
package test_ui

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/0xYeah/project_template_go/docs"
	"github.com/0xYeah/project_template_go/api/api_common"
	"github.com/0xYeah/project_template_go/api/api_config"
	api_grpc_protobuf "github.com/0xYeah/project_template_go/api/api_grpc/protobuf"
	"github.com/0xYeah/project_template_go/api/api_supported_methods"
	"github.com/0xYeah/project_template_go/config"
	"github.com/coder/websocket"
	"github.com/george012/gtbox/gtbox_log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	defaultTestUIPort  = 13003
	maxProxyBodySize   = 4 << 20
	maxProxyResultSize = 8 << 20

	proxyTransportJSONRPC   = "jsonrpc"
	proxyTransportMCP       = "mcp"
	proxyTransportWebSocket = "websocket"
	proxyTransportGRPC      = "grpc"
)

//go:embed web/dist
var embeddedWeb embed.FS

var loadTestWebOnce sync.Once

type apiMethod struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

type webConfig struct {
	JSONRPCEndpoint   string      `json:"jsonRpcEndpoint"`
	MCPEndpoint       string      `json:"mcpEndpoint"`
	WebSocketEndpoint string      `json:"webSocketEndpoint"`
	GRPCEndpoint      string      `json:"grpcEndpoint"`
	Methods           []apiMethod `json:"methods"`
	Project           projectInfo `json:"project"`
}

type projectInfo struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	BundleID string `json:"bundleId"`
	RunMode  string `json:"runMode"`
}

type proxyRequest struct {
	Transport string            `json:"transport"`
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
}

type proxyResponse struct {
	Status        int                 `json:"status"`
	StatusText    string              `json:"statusText"`
	Headers       map[string][]string `json:"headers"`
	Body          string              `json:"body"`
	DurationMS    int64               `json:"durationMs"`
	Size          int                 `json:"size"`
	WasTruncated  bool                `json:"wasTruncated"`
	ProtocolError bool                `json:"protocolError,omitempty"`
}

// LoadTestWeb builds and starts the local API test console.
func LoadTestWeb() {
	loadTestWebOnce.Do(func() {
		webFS, err := loadWebFS()
		if err != nil {
			gtbox_log.LogErrorf("Failed to load test UI: %v", err)
			return
		}

		listener, err := listen()
		if err != nil {
			gtbox_log.LogErrorf("Failed to start test UI: %v", err)
			return
		}

		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/config", handleConfig)
		mux.HandleFunc("POST /api/proxy", handleProxy)
		mux.HandleFunc("GET /docs_api", func(writer http.ResponseWriter, request *http.Request) {
			handleAPIDocPage(writer, request, webFS)
		})

		// dist 内存在的静态资源正常服务；其余路径与 API 侧一致返回 Home 中性空响应，
		// 不输出 FileServer 默认的 "404 page not found" 反馈。
		fileServer := http.FileServer(http.FS(webFS))
		mux.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			name := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
			if name != "" {
				if _, err := fs.Stat(webFS, name); err != nil {
					api_common.HomeHandler(writer, request)
					return
				}
			}
			fileServer.ServeHTTP(writer, request)
		}))

		server := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      70 * time.Second,
			IdleTimeout:       60 * time.Second,
		}

		go func() {
			gtbox_log.LogInfof("Test UI Run On  [http://%s]", listener.Addr().String())
			if serveErr := server.Serve(listener); serveErr != nil &&
				!errors.Is(serveErr, http.ErrServerClosed) {
				gtbox_log.LogErrorf("Test UI stopped unexpectedly: %v", serveErr)
			}
		}()
	})
}

func loadWebFS() (fs.FS, error) {
	webDir := filepath.Join("test_ui", "web")
	packageFile := filepath.Join(webDir, "package.json")
	if _, err := os.Stat(packageFile); err == nil {
		cacheDir := filepath.Join("tmp", "test-ui-bun-cache")
		tempDir := filepath.Join("tmp", "test-ui-bun-tmp")
		if mkdirErr := os.MkdirAll(cacheDir, 0755); mkdirErr != nil {
			return nil, fmt.Errorf("create test UI cache directory: %w", mkdirErr)
		}
		if mkdirErr := os.MkdirAll(tempDir, 0755); mkdirErr != nil {
			return nil, fmt.Errorf("create test UI temp directory: %w", mkdirErr)
		}

		cacheDir, err = filepath.Abs(cacheDir)
		if err != nil {
			return nil, fmt.Errorf("resolve test UI cache directory: %w", err)
		}
		tempDir, err = filepath.Abs(tempDir)
		if err != nil {
			return nil, fmt.Errorf("resolve test UI temp directory: %w", err)
		}

		command := exec.Command("bun", "run", "build")
		command.Dir = webDir
		command.Env = append(
			os.Environ(),
			"TMPDIR="+tempDir,
			"BUN_INSTALL_CACHE_DIR="+cacheDir,
			"XDG_CACHE_HOME="+cacheDir,
		)
		output, buildErr := command.CombinedOutput()
		if buildErr != nil {
			return nil, fmt.Errorf("build test UI: %w: %s", buildErr, strings.TrimSpace(string(output)))
		}

		distDir := filepath.Join(webDir, "dist")
		if _, statErr := os.Stat(filepath.Join(distDir, "index.html")); statErr != nil {
			return nil, fmt.Errorf("test UI build did not create index.html: %w", statErr)
		}
		return os.DirFS(distDir), nil
	}

	return fs.Sub(embeddedWeb, "web/dist")
}

func listen() (net.Listener, error) {
	port := defaultTestUIPort
	if api_config.CurrentApiCfg != nil {
		if api_config.CurrentApiCfg.APICfgJsonRPC != nil &&
			api_config.CurrentApiCfg.APICfgJsonRPC.Port >= port {
			port = api_config.CurrentApiCfg.APICfgJsonRPC.Port + 1
		}
		if api_config.CurrentApiCfg.APICfgMCP != nil &&
			api_config.CurrentApiCfg.APICfgMCP.Port >= port {
			port = api_config.CurrentApiCfg.APICfgMCP.Port + 1
		}
	}
	if port < 1 || port > 65535 {
		port = defaultTestUIPort
	}

	for currentPort := port; currentPort <= 65535; currentPort++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", currentPort))
		if err == nil {
			return listener, nil
		}
	}
	return nil, fmt.Errorf("no available port from %d to 65535", port)
}

const apiDocSourcePlaceholder = "{{API_DOC_SOURCE}}"

// handleAPIDocPage 把对外方法文档注入 docs 页返回；对外不提供任何原始文件路由。
// 文档内容在仓库 checkout 内优先读磁盘（编辑即生效），离仓运行时回落到随包 embed 的副本；
// 任一失败路径与 API 侧一致，返回 Home 中性空响应，不输出任何提示信息。
func handleAPIDocPage(writer http.ResponseWriter, request *http.Request, webFS fs.FS) {
	page, err := fs.ReadFile(webFS, "docs_api.html")
	if err != nil {
		api_common.HomeHandler(writer, request)
		return
	}
	markdown, err := os.ReadFile(filepath.Join("docs", "api_methods.md"))
	if err != nil {
		markdown, err = docs.APIDocFS.ReadFile("api_methods.md")
	}
	if err != nil {
		api_common.HomeHandler(writer, request)
		return
	}
	// json.Marshal 生成合法 JS 字符串字面量并转义 <、>、&，注入 script 安全。
	sourceLiteral, err := json.Marshal(string(markdown))
	if err != nil {
		api_common.HomeHandler(writer, request)
		return
	}
	rendered := strings.Replace(string(page), apiDocSourcePlaceholder, string(sourceLiteral), 1)
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write([]byte(rendered))
}

func handleConfig(writer http.ResponseWriter, _ *http.Request) {
	webCfg := webConfig{
		JSONRPCEndpoint:   "http://127.0.0.1:13001",
		MCPEndpoint:       "http://127.0.0.1:13002",
		WebSocketEndpoint: "ws://127.0.0.1:13004",
		GRPCEndpoint:      "grpc://127.0.0.1:13005",
		Project: projectInfo{
			Name:     config.ProjectName,
			Version:  config.ProjectVersion,
			BundleID: config.ProjectBundleID,
			RunMode:  "Unknown",
		},
	}
	if config.CurrentApp != nil {
		webCfg.Project.RunMode = config.CurrentApp.CurrentRunMode.String()
	}
	if api_config.CurrentApiCfg != nil {
		if api_config.CurrentApiCfg.APICfgJsonRPC != nil {
			webCfg.JSONRPCEndpoint = fmt.Sprintf(
				"http://127.0.0.1:%d",
				api_config.CurrentApiCfg.APICfgJsonRPC.Port,
			)
		}
		if api_config.CurrentApiCfg.APICfgMCP != nil {
			webCfg.MCPEndpoint = fmt.Sprintf(
				"http://127.0.0.1:%d",
				api_config.CurrentApiCfg.APICfgMCP.Port,
			)
		}
		if api_config.CurrentApiCfg.APICfgWebSocket != nil {
			webCfg.WebSocketEndpoint = fmt.Sprintf(
				"ws://127.0.0.1:%d",
				api_config.CurrentApiCfg.APICfgWebSocket.Port,
			)
		}
		if api_config.CurrentApiCfg.APICfgGRPC != nil {
			webCfg.GRPCEndpoint = fmt.Sprintf(
				"grpc://127.0.0.1:%d",
				api_config.CurrentApiCfg.APICfgGRPC.Port,
			)
		}
	}

	for _, method := range api_supported_methods.Methods() {
		webCfg.Methods = append(webCfg.Methods, apiMethod{
			Name:        method.Name,
			Description: method.Description,
			InputSchema: method.InputSchema,
		})
	}
	writeJSON(writer, http.StatusOK, webCfg)
}

func handleProxy(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxProxyBodySize)
	var proxyReq proxyRequest
	if err := json.NewDecoder(request.Body).Decode(&proxyReq); err != nil {
		writeProxyError(writer, http.StatusBadRequest, fmt.Errorf("invalid request: %w", err))
		return
	}

	transport := strings.ToLower(strings.TrimSpace(proxyReq.Transport))
	switch transport {
	case proxyTransportWebSocket:
		ctx, cancel := context.WithTimeout(request.Context(), 60*time.Second)
		defer cancel()
		response, err := proxyWebSocket(ctx, proxyReq)
		if err != nil {
			writeProxyError(writer, http.StatusBadGateway, err)
			return
		}
		writeJSON(writer, http.StatusOK, response)
		return
	case proxyTransportGRPC:
		ctx, cancel := context.WithTimeout(request.Context(), 60*time.Second)
		defer cancel()
		response, err := proxyGRPC(ctx, proxyReq)
		if err != nil {
			writeProxyError(writer, http.StatusBadGateway, err)
			return
		}
		writeJSON(writer, http.StatusOK, response)
		return
	case "", proxyTransportJSONRPC, proxyTransportMCP:
	default:
		writeProxyError(
			writer,
			http.StatusBadRequest,
			fmt.Errorf("unsupported transport: %s", proxyReq.Transport),
		)
		return
	}

	targetURL, err := url.Parse(proxyReq.URL)
	if err != nil || !isLoopbackURL(targetURL) {
		writeProxyError(
			writer,
			http.StatusBadRequest,
			errors.New("only http(s) loopback URLs are allowed"),
		)
		return
	}

	method := strings.ToUpper(strings.TrimSpace(proxyReq.Method))
	if method == "" {
		method = http.MethodPost
	}
	outboundRequest, err := http.NewRequestWithContext(
		request.Context(),
		method,
		targetURL.String(),
		strings.NewReader(proxyReq.Body),
	)
	if err != nil {
		writeProxyError(writer, http.StatusBadRequest, err)
		return
	}
	for name, value := range proxyReq.Headers {
		if !strings.EqualFold(name, "Host") && !strings.EqualFold(name, "Content-Length") {
			outboundRequest.Header.Set(name, value)
		}
	}

	startedAt := time.Now()
	response, err := proxyHTTPClient().Do(outboundRequest)
	duration := time.Since(startedAt).Milliseconds()
	if err != nil {
		writeProxyError(writer, http.StatusBadGateway, err)
		return
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxProxyResultSize+1))
	if err != nil {
		writeProxyError(writer, http.StatusBadGateway, err)
		return
	}
	truncated := len(body) > maxProxyResultSize
	if truncated {
		body = body[:maxProxyResultSize]
	}

	writeJSON(writer, http.StatusOK, proxyResponse{
		Status:       response.StatusCode,
		StatusText:   response.Status,
		Headers:      response.Header,
		Body:         string(body),
		DurationMS:   duration,
		Size:         len(body),
		WasTruncated: truncated,
	})
}

func proxyWebSocket(ctx context.Context, proxyReq proxyRequest) (proxyResponse, error) {
	targetURL, err := url.Parse(proxyReq.URL)
	if err != nil || !isLoopbackWebSocketURL(targetURL) {
		return proxyResponse{}, errors.New(
			"only ws(s) loopback URLs are allowed",
		)
	}

	headers := http.Header{}
	for name, value := range proxyReq.Headers {
		if isReservedProxyHeader(name) ||
			strings.HasPrefix(strings.ToLower(name), "sec-websocket-") {
			continue
		}
		headers.Set(name, value)
	}

	startedAt := time.Now()
	connection, handshakeResponse, err := websocket.Dial(
		ctx,
		targetURL.String(),
		&websocket.DialOptions{
			HTTPClient: proxyHTTPClient(),
			HTTPHeader: headers,
		},
	)
	if err != nil {
		return proxyResponse{}, fmt.Errorf("connect WebSocket: %w", err)
	}
	defer connection.Close(websocket.StatusNormalClosure, "")
	connection.SetReadLimit(maxProxyResultSize + 1)

	if err := connection.Write(
		ctx,
		websocket.MessageText,
		[]byte(proxyReq.Body),
	); err != nil {
		return proxyResponse{}, fmt.Errorf("write WebSocket request: %w", err)
	}
	messageType, body, err := connection.Read(ctx)
	if err != nil {
		return proxyResponse{}, fmt.Errorf("read WebSocket response: %w", err)
	}
	if messageType != websocket.MessageText {
		return proxyResponse{}, errors.New(
			"WebSocket response must be a text message",
		)
	}

	truncated := len(body) > maxProxyResultSize
	if truncated {
		body = body[:maxProxyResultSize]
	}
	responseHeaders := map[string][]string{}
	if handshakeResponse != nil {
		responseHeaders = handshakeResponse.Header
	}
	return proxyResponse{
		Status:       http.StatusSwitchingProtocols,
		StatusText:   "101 Switching Protocols",
		Headers:      responseHeaders,
		Body:         string(body),
		DurationMS:   time.Since(startedAt).Milliseconds(),
		Size:         len(body),
		WasTruncated: truncated,
	}, nil
}

func proxyGRPC(ctx context.Context, proxyReq proxyRequest) (proxyResponse, error) {
	address, err := parseLoopbackGRPCEndpoint(proxyReq.URL)
	if err != nil {
		return proxyResponse{}, err
	}

	connection, err := grpc.NewClient(
		"passthrough:///"+address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialLoopback),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(maxProxyBodySize),
			grpc.MaxCallRecvMsgSize(maxProxyResultSize+1),
		),
	)
	if err != nil {
		return proxyResponse{}, fmt.Errorf("create gRPC client: %w", err)
	}
	defer connection.Close()

	if len(proxyReq.Headers) > 0 {
		outgoingMetadata := map[string]string{}
		for name, value := range proxyReq.Headers {
			if !isReservedProxyHeader(name) {
				outgoingMetadata[strings.ToLower(name)] = value
			}
		}
		ctx = metadata.NewOutgoingContext(ctx, metadata.New(outgoingMetadata))
	}

	var responseHeaders metadata.MD
	var responseTrailers metadata.MD
	request := &api_grpc_protobuf.CallRequest{}
	if err = protojson.Unmarshal([]byte(proxyReq.Body), request); err != nil {
		return proxyResponse{}, fmt.Errorf("decode gRPC request: %w", err)
	}
	startedAt := time.Now()
	response, err := api_grpc_protobuf.NewAPIServiceClient(connection).Call(
		ctx,
		request,
		grpc.Header(&responseHeaders),
		grpc.Trailer(&responseTrailers),
	)
	if err != nil {
		protocolStatus := grpcstatus.Convert(err)
		body, encodeErr := protojson.Marshal(protocolStatus.Proto())
		if encodeErr != nil {
			return proxyResponse{}, fmt.Errorf("encode gRPC status: %w", encodeErr)
		}
		return proxyResponse{
			Status:        http.StatusOK,
			StatusText:    "gRPC " + protocolStatus.Code().String(),
			Headers:       mergeGRPCMetadata(responseHeaders, responseTrailers),
			Body:          string(body),
			DurationMS:    time.Since(startedAt).Milliseconds(),
			Size:          len(body),
			ProtocolError: true,
		}, nil
	}

	body, err := protojson.Marshal(response)
	if err != nil {
		return proxyResponse{}, fmt.Errorf("encode gRPC response: %w", err)
	}
	truncated := len(body) > maxProxyResultSize
	if truncated {
		body = body[:maxProxyResultSize]
	}
	return proxyResponse{
		Status:       http.StatusOK,
		StatusText:   "gRPC OK",
		Headers:      mergeGRPCMetadata(responseHeaders, responseTrailers),
		Body:         string(body),
		DurationMS:   time.Since(startedAt).Milliseconds(),
		Size:         len(body),
		WasTruncated: truncated,
	}, nil
}

func proxyHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if !isLoopbackURL(request.URL) {
				return errors.New("redirect to a non-loopback URL is not allowed")
			}
			return nil
		},
		Transport: &http.Transport{
			DialContext: func(
				ctx context.Context,
				network string,
				address string,
			) (net.Conn, error) {
				host, _, err := net.SplitHostPort(address)
				if err != nil || !isLoopbackHost(host) {
					return nil, errors.New("only loopback targets are allowed")
				}
				dialer := net.Dialer{Timeout: 10 * time.Second}
				return dialer.DialContext(ctx, network, address)
			},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		},
	}
}

func parseLoopbackGRPCEndpoint(rawEndpoint string) (string, error) {
	endpoint := strings.TrimSpace(rawEndpoint)
	if !strings.Contains(endpoint, "://") {
		endpoint = "grpc://" + endpoint
	}
	target, err := url.Parse(endpoint)
	if err != nil ||
		target.Scheme != "grpc" ||
		target.Path != "" ||
		!isLoopbackHost(target.Hostname()) {
		return "", errors.New(
			"only grpc loopback endpoints are allowed",
		)
	}
	if _, _, err := net.SplitHostPort(target.Host); err != nil {
		return "", errors.New(
			"gRPC endpoint must include a valid port",
		)
	}
	return target.Host, nil
}

func dialLoopback(ctx context.Context, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || !isLoopbackHost(host) {
		return nil, errors.New("only loopback targets are allowed")
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(ctx, "tcp", address)
}

func mergeGRPCMetadata(headers metadata.MD, trailers metadata.MD) map[string][]string {
	result := map[string][]string{}
	for name, values := range headers {
		result[name] = append([]string(nil), values...)
	}
	for name, values := range trailers {
		result["trailer-"+name] = append([]string(nil), values...)
	}
	return result
}

func isReservedProxyHeader(name string) bool {
	return strings.EqualFold(name, "Host") ||
		strings.EqualFold(name, "Content-Length") ||
		strings.EqualFold(name, "Connection") ||
		strings.EqualFold(name, "Upgrade")
}

func isLoopbackURL(target *url.URL) bool {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") {
		return false
	}
	return isLoopbackHost(target.Hostname())
}

func isLoopbackWebSocketURL(target *url.URL) bool {
	if target == nil ||
		(target.Scheme != "ws" && target.Scheme != "wss") {
		return false
	}
	return isLoopbackHost(target.Hostname())
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func writeProxyError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, map[string]string{"error": err.Error()})
}

func writeJSON(writer http.ResponseWriter, status int, value interface{}) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		gtbox_log.LogErrorf("Failed to encode test UI response: %v", err)
	}
}
