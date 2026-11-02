// Package project_template_go /main.go
package main

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/0xdevelop/project_template_go/api"
	"github.com/0xdevelop/project_template_go/common"
	"github.com/0xdevelop/project_template_go/config"
	"github.com/0xdevelop/project_template_go/custom_cmd"
	"github.com/0xdevelop/project_template_go/db"
	"github.com/0xdevelop/project_template_go/policy"
	"github.com/0xdevelop/project_template_go/test_ui"
	"github.com/george012/gtbox"
	"github.com/george012/gtbox/gtbox_cmd"
	"github.com/george012/gtbox/gtbox_log"
)

var (
	mRunMode       = ""
	mGitCommitHash = ""
	mGitCommitTime = ""
	mPackageOS     = ""
	mPackageTime   = ""
	mGoVersion     = ""
)

func setupApp() {
	runMode := gtbox.RunModeDebug
	switch mRunMode {
	case "debug":
		runMode = gtbox.RunModeDebug
	case "test":
		runMode = gtbox.RunModeTest
	case "release":
		runMode = gtbox.RunModeRelease
	default:
		runMode = gtbox.RunModeDebug
	}

	config.CurrentApp = config.NewApp(
		config.ProjectName,
		config.ProjectBundleID,
		fmt.Sprintf("%s service",
			config.ProjectName,
		),
		runMode,
	)

	if config.CurrentApp.CurrentRunMode == gtbox.RunModeDebug {
		cmdMap := map[string]string{
			"git_commit_hash": "git show -s --format=%H",
			"git_commit_time": "git show -s --format=\"%ci\" | cut -d ' ' -f 1,2 | sed 's/ /_/'",
			"build_os":        "go env GOOS",
			"go_version":      "go version | awk '{print $3}'",
		}
		cmdRes := gtbox_cmd.RunWith(cmdMap)

		if cmdRes != nil {
			mGitCommitHash = cmdRes["git_commit_hash"]
			mGitCommitTime = cmdRes["git_commit_time"]
			mPackageOS = cmdRes["build_os"]
			mGoVersion = cmdRes["go_version"]
			mPackageTime = time.Now().UTC().Format("2006-01-02_15:04:05")
		}
	}

	config.CurrentApp.GitCommitHash = mGitCommitHash
	config.CurrentApp.GitCommitTime = mGitCommitTime
	config.CurrentApp.GoVersion = mGoVersion
	config.CurrentApp.PackageOS = mPackageOS
	config.CurrentApp.PackageTime = mPackageTime

	custom_cmd.HandleCustomCmds(os.Args, config.CurrentApp)

	gtbox.SetupGTBox(config.CurrentApp.AppName,
		config.CurrentApp.CurrentRunMode,
		config.CurrentApp.AppLogPath,
		30,
		gtbox_log.GTLogSaveHours,
		config.CurrentApp.HTTPRequestTimeOut,
	)
}

func main() {
	runtime.LockOSThread()

	setupApp()

	defer common.PanicHandler()

	if config.CurrentApp.CurrentRunMode == gtbox.RunModeDebug {
		config.CurrentApp.AppConfigFilePath = "./example_files/config_local.yaml"
		//config.CurrentApp.AppConfigFilePath = "./example_files/config_local_outside.yaml"
	} else {
		config.CurrentApp.AppConfigFilePath = "./conf/config.yaml"
	}

	config.SyncConfigFile(func(err error) {
		gtbox_log.LogErrorf("[sync config eeror]:%v", err)
	})
	if err := db.OpenStorage(); err != nil {
		gtbox_log.LogErrorf("storage open failed: %v", err)
		return
	}
	if err := db.GormAutoMigrate(); err != nil {
		gtbox_log.LogErrorf("gorm AutoMigrate failed: %v", err)
		if closeErr := db.CloseStorage(); closeErr != nil {
			gtbox_log.LogErrorf("storage close failed: %v", closeErr)
		}
		return
	}
	gtbox_log.LogInfof("storage migrated")

	// 启动维护调度域内部异步大循环
	policy.PolicyServicesStart()

	if err := config.GlobalConfig.ApiCfg.Validate(); err != nil {
		gtbox_log.LogErrorf("api config invalid: %v; refusing to start", err)
		if closeErr := db.CloseStorage(); closeErr != nil {
			gtbox_log.LogErrorf("storage close failed: %v", closeErr)
		}
		return
	}
	api.StartAPIServices(config.GlobalConfig.ApiCfg)

	if config.CurrentApp.CurrentRunMode != gtbox.RunModeRelease {
		test_ui.LoadTestWeb()
	}
	common.LoadSigHandle(func() {
		api.StopApiServices()
		if err := db.CloseStorage(); err != nil {
			gtbox_log.LogErrorf("storage close failed: %v", err)
		}
	}, nil)

}
