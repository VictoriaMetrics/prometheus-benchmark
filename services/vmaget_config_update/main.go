package main

import (
	"context"
	"flag"
	"time"

	"prometheus-benchmark/services/vmaget_config_update/controllers"
	"prometheus-benchmark/services/vmaget_config_update/models"
	"prometheus-benchmark/services/vmaget_config_update/runner"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var (
	listenAddr           = flag.String("http.listenAddr", ":8436", "http service listener addr")
	configUpdateInterval = flag.Duration("configUpdateInterval", time.Second*10, "interval when config will be updated")
)

func main() {
	envflag.Parse()
	logger.Init()
	ctx := context.Background()
	logger.Infof("starting config updater service")
	r := runner.New(func(ctx context.Context) error {
		return models.Update()
	})
	if err := r.Run(*configUpdateInterval); err != nil {
		logger.Fatalf("failed to run vmagent config updater: %s", err)
	}

	go httpserver.Serve(*listenAddr, controllers.Init(ctx))

	logger.Infof("listening on: %v", *listenAddr)
	procutil.WaitForSigterm()
	logger.Infof("got stop signal, shutting down service.")

	if err := r.Close(); err != nil {
		logger.Errorf("cannot stop vmagent updater: %s", err)
	}

	if err := httpserver.Stop(*listenAddr); err != nil {
		logger.Fatalf("error stop server: %s", err)
	}
}
