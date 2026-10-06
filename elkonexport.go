package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dronm/elkonexport/config"
	"github.com/dronm/elkonexport/logger"
)

func main() {
	var cfgName string
	if len(os.Args) >= 2 {
		cfgName = os.Args[1] //config passed as argument
	} else {
		exePath, err := os.Executable() // get full path to current executable
		if err != nil {
			// fmt.Fprintf(os.Stderr, "failed to get executable path: %v\n", err)
			dumpToFile(fmt.Sprintf("failed to get executable path: %v", err))
			os.Exit(2)
		}
		exeName := filepath.Base(exePath)
		name := exeName[:len(exeName)-len(filepath.Ext(exeName))] // strip .exe, .bin, etc.
		cfgName = name + ".json"
	}

	// Load config
	cfgMgr := config.NewManager(cfgName)
	if err := cfgMgr.Load(); err != nil {
		dumpToFile(fmt.Sprintf("failed to load config: %v", err))
		os.Exit(2)
	}
	cfg := cfgMgr.Get()

	// Init logger
	logger.Initialize(cfg.LogLevel, cfg.LogFileName)
	logger.Logger.Infof("starting elkonexport with config: %s", cfgMgr.FileName)
	defer logger.FlushLogFile()

	// Create app
	app, err := NewApp(cfgMgr)
	if err != nil {
		logger.Logger.Fatalf("failed to create app: %v", err)
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		logger.Logger.Info("shutdown signal received")
		app.Shutdown()
	}()

	// Run app
	if err := app.Run(); err != nil {
		logger.Logger.Fatalf("app error: %v", err)
	}
}

func dumpToFile(s string) {
	dir, _ := os.Getwd()
	f, _ := os.OpenFile(filepath.Join(dir, "elkonexport.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()

	fmt.Println(f, s)
}
