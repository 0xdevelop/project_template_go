package api_jsonRPC

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/0xdevelop/project_template_go/api/api_common"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_config_jsonRPC"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_handler"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/gorilla/mux"
)

var httpServer *http.Server

func StartAPIServiceWithJsonRPC(apiCfgJsonRPC *api_config_jsonRPC.APIConfigJsonRPC) {
	api_config_jsonRPC.CurrentAPICfgJsonRPC = apiCfgJsonRPC

	if apiCfgJsonRPC.Port < 1 || apiCfgJsonRPC.Port > 65535 {
		gtbox_log.LogErrorf("api port must be between 1 and 65535")
		return
	}

	muxRouter := mux.NewRouter()
	//muxRouter.Use(api_jsonRPC_handler.Middleware) // 使用中间件
	muxRouter.HandleFunc("/", api_common.HomeHandler).Methods("GET")
	muxRouter.HandleFunc("/", api_jsonRPC_handler.APIJsonRPCHandler).Methods("POST")
	muxRouter.HandleFunc("/robots.txt", api_common.RobotsHandler).Methods("GET")
	muxRouter.NotFoundHandler = http.HandlerFunc(api_common.HomeHandler)
	muxRouter.MethodNotAllowedHandler = http.HandlerFunc(api_common.HomeHandler)

	addr := fmt.Sprintf("%s:%d", "0.0.0.0", api_config_jsonRPC.CurrentAPICfgJsonRPC.Port)

	httpServer = &http.Server{
		Addr:    addr,
		Handler: muxRouter,
	}
	go func() {
		gtbox_log.LogInfof("API server Run On  [%s]", fmt.Sprintf("http://127.0.0.1:%d", api_config_jsonRPC.CurrentAPICfgJsonRPC.Port))
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			gtbox_log.LogErrorf("Failed to start HTTP server: %v\n", err)
		}
	}()

}

func StopApiServiceWithJsonRPC() {
	if httpServer == nil {
		gtbox_log.LogInfof("API server is not running")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	gtbox_log.LogInfof("Shutting down API server...")
	if err := httpServer.Shutdown(ctx); err != nil {
		gtbox_log.LogErrorf("Error shutting down API server: %v\n", err)
	} else {
		gtbox_log.LogInfof("API server stopped successfully")
	}
}
