package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/0xdevelop/project_template_go/common/app_path"
	"github.com/0xdevelop/project_template_go/common/resource"
	"github.com/0xdevelop/project_template_go/config"
	"os"
	"os/signal"
	"syscall"
)

// Product resource command runs before normal app setup: no server/storage/systemd startup.
func resourceCommand(args []string) int {
	root, err := app_path.ExecutableDir()
	if err != nil {
		return 1
	}
	fs := flag.NewFlagSet("resource", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	runtimePath := fs.String("runtime", "", "expected official wrapper path for verification")
	file := fs.String("config", "conf/config.yaml", "configuration file, relative to executable directory")
	if fs.Parse(args) != nil {
		return 2
	}
	path, err := app_path.Resolve(root, *file)
	if err != nil {
		return 1
	}
	if err = config.LoadConfig(path); err != nil {
		fmt.Fprintln(os.Stderr, "cannot load resource configuration (contents suppressed)")
		return 1
	}
	if rest := fs.Args(); len(rest) > 1 && rest[0] == "verify" {
		if *runtimePath != "" {
			if err = config.VerifyResourceRuntimeBinding(rest[1], *runtimePath); err != nil {
				fmt.Fprintln(os.Stderr, "configured resource/runtime binding mismatch")
				return 1
			}
		}
		if err = config.VerifyResourceRuntime(rest[1]); err != nil {
			fmt.Fprintln(os.Stderr, "runtime revision not verified; Python was not started")
			return 1
		}
	}
	interactive := resource.IsTerminal(os.Stdin)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return resource.RunCLI(ctx, resource.Controller{Root: root, Sets: config.ConfiguredResourceSets()}, fs.Args(), os.Stdin, os.Stdout, os.Stderr, interactive)
}
