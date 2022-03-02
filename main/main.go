package main

import (
	smReport "github.com/r4g3baby/sm-report"
	"github.com/r4g3baby/sm-report/config"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	smReport.StartBot()
	defer smReport.ShutdownBot()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, os.Interrupt)
	sig := <-shutdownSignal

	config.Logger.Debugw("received shutdown signal",
		"signal", sig,
	)
}
