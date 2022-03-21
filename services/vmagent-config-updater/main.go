package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"time"

	"prometheus-benchmark/services/vmagent-config-updater/models"
	"prometheus-benchmark/services/vmagent-config-updater/runner"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

const (
	configPath = "/api/v1/config"
)

var (
	listenAddr                = flag.String("http.listenAddr", ":8436", "address with port for listening for HTTP requests")
	configUpdateInterval      = flag.Duration("config.updateInterval", time.Second*10, "How frequent to refresh the configuration. See also 'config.targetsToUpdatePercentage'")
	targetsToUpdatePercentage = flag.Int("config.targetsToUpdatePercentage", 10, "Percentage of the targets which would be updated with unique label on the next configuration update. Non-zero value will result in constant Churn Rate")
	targetsCount              = flag.Int("config.targetsCount", 1000, "Defines how many scrape targets to generate in scrape config. Each target will have the same address defined by 'config.targetAddr' but each with unique label.")
	targetAddr                = flag.String("config.targetAddr", "vm-benchmark-exporter.default.svc:9102", "Address with port to use as target address in scrape config")
	scrapeInterval            = flag.Duration("config.scrapeInterval", time.Second*5, "Defines how frequently to scrape targets")
	jobName                   = flag.String("config.jobName", "node_exporter", "Defines the job name for scrape targets")
)

func main() {
	envflag.Parse()
	logger.Init()
	logger.Infof("starting config updater service")

	c := models.InitConfigManager(models.NewConfig(
		models.WithTargetCount(*targetsCount),
		models.WithTargetsToUpdatePercentage(*targetsToUpdatePercentage),
		models.WithTargetAddr(*targetAddr),
		models.WithScrapeInterval(*scrapeInterval),
		models.WithJobName(*jobName)))

	r := runner.New(func(ctx context.Context) error {
		return c.Update()
	})

	if err := r.Run(*configUpdateInterval); err != nil {
		logger.Fatalf("failed to run vmagent config updater: %s", err)
	}

	go httpserver.Serve(*listenAddr, requestHandler)

	logger.Infof("listening on: %v", *listenAddr)
	procutil.WaitForSigterm()
	logger.Infof("got stop signal, shutting down service.")

	if err := r.Close(); err != nil {
		logger.Errorf("failed to stop config updater: %s", err)
	}

	if err := httpserver.Stop(*listenAddr); err != nil {
		logger.Fatalf("failed to stop the HTTP: %s", err)
	}
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet {
		return respondWithError(w, r, http.StatusBadRequest, fmt.Errorf("unsupported HTTP method %q", r.Method))
	}
	if r.URL.Path != configPath {
		return respondWithError(w, r, http.StatusBadRequest, fmt.Errorf("unsupported path %q", r.URL.Path))
	}
	if _, err := w.Write(models.GetConfig()); err != nil {
		logger.Errorf("failed to write response: %s", err)
		return false
	}
	w.Header().Set("Content-Type", "plain/text")
	w.WriteHeader(http.StatusOK)
	return true
}

func respondWithError(w http.ResponseWriter, r *http.Request, statusCode int, err error) bool {
	logger.Errorf(err.Error(), r.URL.Path)
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(err.Error()))
	return true
}
