package frontend

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/0xdevelop/project_template_go/frontend/frontend_config"
	"github.com/george012/gtbox/gtbox_log"
)

//go:embed all:dist
var frontendFiles embed.FS

// FrontendServicesStart 按配置启动前端；未启用时返回 nil，由调用方关闭已启动的 HTTP 服务。
func FrontendServicesStart(cfg *frontend_config.FrontendConfig) (*http.Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg == nil {
		gtbox_log.LogInfof("Frontend disabled: frontend_cfg is not configured")
		return nil, nil
	}
	if !cfg.Enabled {
		gtbox_log.LogInfof("Frontend disabled: frontend_cfg.enabled=false")
		return nil, nil
	}
	webFS, err := fs.Sub(frontendFiles, "dist")
	if err != nil {
		return nil, fmt.Errorf("load frontend assets: %w", err)
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(cfg.BindAddress, strconv.Itoa(cfg.Port)))
	if err != nil {
		return nil, fmt.Errorf("listen frontend: %w", err)
	}
	server := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           frontendStaticHandler(webFS),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		gtbox_log.LogInfof("Frontend Run On  [http://%s]", server.Addr)
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			gtbox_log.LogErrorf("frontend stopped unexpectedly: %v", serveErr)
		}
	}()
	return server, nil
}

func frontendStaticHandler(webFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(webFS))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			return
		}
		name := strings.TrimPrefix(path.Clean(request.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		info, err := fs.Stat(webFS, name)
		if err != nil || !info.Mode().IsRegular() {
			return
		}
		fileServer.ServeHTTP(writer, request)
	})
}
