package main

import (
	"context"
	"flag"
	"time"

	"prometheus-benchmark/services/vmagent-config-update/controllers"
	"prometheus-benchmark/services/vmagent-config-update/models"
	"prometheus-benchmark/services/vmagent-config-update/runner"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

var (
	listenAddr           = flag.String("http.listenAddr", ":8436", "http service listener addr")
	configUpdateInterval = flag.Duration("configUpdateInterval", time.Second*10, "interval when config will be updated")
	targetsCount         = flag.Int("targetsCount", 5, "targetsCount defines how many copies of nodeexporter to add as scrape targets. affects metrics volume and cardinality")
	targetName           = flag.String("targetName", "vm-benchmark-exporter.default.svc:9102", "target name with host and port name")
	scrapInterval        = flag.Duration("scrapInterval", time.Second*5, "defines how frequently scrape targets")
	vmagentListenAddr    = flag.String("http.vmagentListenAddr", "localhost:8429", "http vmagent service listen addr")
)

func main() {
	envflag.Parse()
	logger.Init()
	ctx := context.Background()
	logger.Infof("starting config updater service")

	c := models.InitConfigManager(models.NewConfig(
		models.WithGlobalConfig(*scrapInterval),
		models.WithScrapeConfig(*targetsCount, *targetName, *vmagentListenAddr)))

	r := runner.New(func(ctx context.Context) error {
		return c.Update()
	})

	if err := r.Run(*configUpdateInterval); err != nil {
		logger.Fatalf("failed to run vmagent config updater: %s", err)
	}

	go httpserver.Serve(*listenAddr, controllers.Init(ctx, *vmagentListenAddr))

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
