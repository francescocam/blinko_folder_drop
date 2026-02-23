package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"blinko-folder-drop/internal/config"
	"blinko-folder-drop/internal/platform"
	"blinko-folder-drop/internal/service"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: blinko-folder-drop <run|validate-config|version> [--config path]")
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version":
		fmt.Println(version)
	case "validate-config":
		validateConfigCmd(os.Args[2:])
	case "run":
		runCmd(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(2)
	}
}

func validateConfigCmd(args []string) {
	fs := flag.NewFlagSet("validate-config", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file")
	_ = fs.Parse(args)

	if *configPath == "" {
		log.Fatal("--config is required")
	}

	if _, err := config.Load(*configPath); err != nil {
		log.Fatalf("config invalid: %v", err)
	}
	fmt.Println("config is valid")
}

func runCmd(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	configPath := fs.String("config", "", "path to config file")
	_ = fs.Parse(args)

	if *configPath == "" {
		log.Fatal("--config is required")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	runner := func(ctx context.Context) error {
		svc, err := service.New(cfg)
		if err != nil {
			return err
		}
		return svc.Run(ctx)
	}

	if runtime.GOOS == "windows" {
		used, err := platform.TryRunWindowsService(version, runner)
		if err != nil {
			log.Fatalf("windows service start: %v", err)
		}
		if used {
			return
		}
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := runner(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("run failed: %v", err)
	}
}
