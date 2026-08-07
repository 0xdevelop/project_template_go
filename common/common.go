package common

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/0xYeah/project_template_go/config"
	"github.com/george012/gtbox"
	"github.com/george012/gtbox/gtbox_coding"
	"github.com/george012/gtbox/gtbox_log"
	"github.com/george012/gtbox/gtbox_net"
	"github.com/george012/gtbox/gtbox_time"
)

var (
	globalTransport *http.Transport
	initOnce        sync.Once
)

func GetGlobalHttpClient(requestTimeout time.Duration) *http.Client {
	initOnce.Do(func() {
		globalTransport = &http.Transport{
			MaxIdleConns:          500,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   requestTimeout,
			ResponseHeaderTimeout: requestTimeout,
			ExpectContinueTimeout: 1 * time.Second,
		}
	})
	return &http.Client{
		Transport: globalTransport,
		Timeout:   requestTimeout,
	}
}

func LoadSigHandle(cleanAction func(), testMethods []func()) {
	if config.CurrentApp != nil && config.CurrentApp.CurrentRunMode == gtbox.RunModeDebug {
		testMethod(testMethods)
	} else if config.CurrentApp == nil {
		testMethod(testMethods)
	}
	// 创建一个信号通道
	chSig := make(chan os.Signal, 1)
	signal.Notify(chSig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGKILL)

	// 阻断主进程等待signal
	asig := <-chSig
	if cleanAction != nil {
		cleanAction()
	}
	gtbox_log.LogInfof("接收到 [%s] 信号，程序即将退出! ", asig)
	willExitHandle()
}

// willExitHandle 异常退出处理
func willExitHandle() {
	gtbox_log.LogInfof("[程序关闭]---[处理缓存数据] ")

	// 退出
	ExitApp()
}

func testMethod(testMethods []func()) {
	line_No := gtbox_coding.GetProjectCodeLines()
	gtbox_log.LogDebugf("项目有效代码总行数: %v", line_No)
	gtbox_log.LogDebugf("当前公网IP: %v", gtbox_net.GTGetPublicIPV4())
	for _, method := range testMethods {
		go method()
		gtbox_log.LogDebugf("开始执行测试方法: %v", method)

	}
}

func ExitApp() {
	// 发送 os.Interrupt 信号以触发正常退出
	p, _ := os.FindProcess(os.Getpid())
	p.Signal(os.Interrupt)
}

// PanicHandler 统一 panic 兜底：输出富日志（时间/上下文/触发点/堆栈）。
// args 接受任意个 string 作为上下文说明；额外传入一个 *error 时（如后台 Worker 需按 panic 落任务终态），
// panic 会转为 error 写入该指针交还调用方。原有 PanicHandler() / PanicHandler("ctx") 调用形态不变。
func PanicHandler(args ...interface{}) {
	err := recover()
	if err != nil {
		var contexts []string
		var errOut *error
		for _, arg := range args {
			switch value := arg.(type) {
			case string:
				contexts = append(contexts, value)
			case *error:
				errOut = value
			}
		}

		at_time := gtbox_time.NowUTC().Format("2006-01-02_15:04:05.000") + " UTC"
		gtbox_log.LogErrorf("====================== Panic Error ======================")
		if len(contexts) > 0 {
			gtbox_log.LogErrorf("[%s] 上下文: %s", at_time, strings.Join(contexts, ", "))
		}
		gtbox_log.LogErrorf("发生时间: [%s]", at_time)
		gtbox_log.LogErrorf("错误信息: [%v]", err)

		// 获取堆栈信息
		stack := debug.Stack()

		// 提取真正触发 panic 的方法、文件和行号
		var methodName, fileName string
		var lineNum int

		pcs := make([]uintptr, 64)
		count := runtime.Callers(2, pcs)
		frames := runtime.CallersFrames(pcs[:count])

		for {
			frame, more := frames.Next()
			isRuntimeFrame := strings.HasPrefix(frame.Function, "runtime.") || strings.HasPrefix(frame.Function, "runtime/")
			isPanicHandler := strings.HasSuffix(frame.Function, ".PanicHandler")

			if frame.Function != "" && !isRuntimeFrame && !isPanicHandler {
				methodName = frame.Function
				fileName = frame.File
				lineNum = frame.Line
				break
			}

			if !more {
				break
			}
		}

		gtbox_log.LogErrorf("触发方法: [%s]", methodName)
		gtbox_log.LogErrorf("所在文件: [%s], 行号: [%d]", fileName, lineNum)
		gtbox_log.LogErrorf("堆栈跟踪:\n%s", string(stack))
		gtbox_log.LogErrorf("=========================================================")

		if errOut != nil {
			*errOut = fmt.Errorf("panic recovered: %v", err)
		}
	}
}
