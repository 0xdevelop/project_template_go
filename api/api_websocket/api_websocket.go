package api_websocket

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/0xdevelop/project_template_go/api/api_common"
	"github.com/0xdevelop/project_template_go/api/api_executer"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_protocol"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_request"
	"github.com/0xdevelop/project_template_go/api/api_jsonRPC/api_jsonRPC_response"
	"github.com/0xdevelop/project_template_go/api/api_websocket/api_config_websocket"
	"github.com/coder/websocket"
	"github.com/george012/gtbox/gtbox_log"
)

const webSocketMaxMessageSize = 4 << 20

var (
	webSocketHTTPServer *http.Server
	webSocketCancel     context.CancelFunc
)

func newWebSocketHandler(
	serverContext context.Context,
	apiCfgWebSocket *api_config_websocket.APIConfigWebSocket,
) http.Handler {
	muxRouter := http.NewServeMux()
	muxRouter.HandleFunc("GET /robots.txt", api_common.RobotsHandler)
	muxRouter.HandleFunc("GET /{$}", func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if !strings.EqualFold(request.Header.Get("Upgrade"), "websocket") {
			api_common.HomeHandler(writer, request)
			return
		}
		serveWebSocket(
			serverContext,
			apiCfgWebSocket,
			writer,
			request,
		)
	})
	muxRouter.HandleFunc("/", api_common.HomeHandler)
	return muxRouter
}

func serveWebSocket(
	serverContext context.Context,
	apiCfgWebSocket *api_config_websocket.APIConfigWebSocket,
	writer http.ResponseWriter,
	request *http.Request,
) {
	connection, err := websocket.Accept(
		writer,
		request,
		&websocket.AcceptOptions{
			OriginPatterns: apiCfgWebSocket.AllowedOrigins,
		},
	)
	if err != nil {
		gtbox_log.LogErrorf("Failed to accept WebSocket connection: %v", err)
		return
	}
	defer connection.CloseNow()
	connection.SetReadLimit(webSocketMaxMessageSize)
	handleWebSocketConnection(
		serverContext,
		connection,
		request.UserAgent(),
	)
}

func handleWebSocketConnection(
	serverContext context.Context,
	connection *websocket.Conn,
	userAgent string,
) {
	ctx, cancel := context.WithCancel(serverContext)
	defer cancel()

	for {
		messageType, payload, readErr := connection.Read(ctx)
		if readErr != nil {
			if ctx.Err() == nil &&
				websocket.CloseStatus(readErr) != websocket.StatusNormalClosure &&
				websocket.CloseStatus(readErr) != websocket.StatusGoingAway {
				gtbox_log.LogErrorf(
					"WebSocket read failed: %v",
					readErr,
				)
			}
			return
		}
		if messageType != websocket.MessageText {
			_ = connection.Close(
				websocket.StatusUnsupportedData,
				"JSON text messages are required",
			)
			return
		}

		responsePayload, notification, processErr := processWebSocketRequest(
			ctx, payload, userAgent,
		)
		if processErr != nil {
			gtbox_log.LogErrorf(
				"WebSocket request processing failed: %v",
				processErr,
			)
			_ = connection.Close(
				websocket.StatusInternalError,
				"request processing failed",
			)
			return
		}
		if notification {
			continue
		}
		if writeErr := connection.Write(
			ctx,
			websocket.MessageText,
			responsePayload,
		); writeErr != nil {
			if ctx.Err() == nil {
				gtbox_log.LogErrorf(
					"WebSocket response write failed: %v",
					writeErr,
				)
			}
			return
		}
	}
}

func processWebSocketRequest(
	ctx context.Context,
	payload []byte,
	userAgent string,
) ([]byte, bool, error) {
	request := &api_jsonRPC_protocol.RPCRequest{}
	if err := api_jsonRPC_request.ParserRequest(payload, request); err != nil {
		responsePayload, encodeErr := api_jsonRPC_response.Encode(nil, err, request.ID)
		return responsePayload, false, encodeErr
	}
	encryptionKey := fmt.Sprintf("%s/%s", userAgent, request.IDString())
	result, err := api_executer.APIExecuter(
		ctx,
		request.Method,
		request.Params,
		encryptionKey,
	)
	if err != nil {
		if !errors.Is(err, api_executer.ErrInvalidCall) {
			return nil, false, err
		}
		responsePayload, encodeErr := api_jsonRPC_response.Encode(
			nil,
			&api_jsonRPC_protocol.RPCError{Code: -32602, Message: err.Error()},
			request.ID,
		)
		return responsePayload, false, encodeErr
	}
	if request.IsNotification() {
		return nil, true, nil
	}
	responsePayload, err := api_jsonRPC_response.Encode(result, nil, request.ID)
	return responsePayload, false, err
}

func StartAPIServiceWithWebSocket(
	apiCfgWebSocket *api_config_websocket.APIConfigWebSocket,
) {
	if apiCfgWebSocket == nil {
		gtbox_log.LogErrorf("WebSocket API config is nil")
		return
	}
	api_config_websocket.CurrentAPICfgWebSocket = apiCfgWebSocket
	if apiCfgWebSocket.Port < 1 || apiCfgWebSocket.Port > 65535 {
		gtbox_log.LogErrorf("WebSocket API port must be between 1 and 65535")
		return
	}

	addr := fmt.Sprintf("0.0.0.0:%d", apiCfgWebSocket.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		gtbox_log.LogErrorf("Failed to start WebSocket server: %v", err)
		return
	}

	serverContext, cancel := context.WithCancel(context.Background())
	server := &http.Server{
		Addr:              addr,
		Handler:           newWebSocketHandler(serverContext, apiCfgWebSocket),
		ReadHeaderTimeout: 5 * time.Second,
	}
	webSocketCancel = cancel
	webSocketHTTPServer = server
	go func() {
		gtbox_log.LogInfof("WebSocket server Run On  [ws://127.0.0.1:%d]", apiCfgWebSocket.Port)
		if serveErr := server.Serve(listener); serveErr != nil &&
			!errors.Is(serveErr, http.ErrServerClosed) {
			gtbox_log.LogErrorf(
				"WebSocket server stopped unexpectedly: %v",
				serveErr,
			)
		}
	}()
}

func StopApiServiceWithWebSocket() {
	server := webSocketHTTPServer
	if server == nil {
		gtbox_log.LogInfof("WebSocket server is not running")
		return
	}
	if webSocketCancel != nil {
		webSocketCancel()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gtbox_log.LogInfof("Shutting down WebSocket server...")
	if err := server.Shutdown(ctx); err != nil {
		gtbox_log.LogErrorf("Error shutting down WebSocket server: %v", err)
	} else {
		gtbox_log.LogInfof("WebSocket server stopped successfully")
	}
	webSocketHTTPServer = nil
	webSocketCancel = nil
}
