package controllers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"prometheus-benchmark/services/vmagent-config-update/models"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	configPath = "/api/v1/config"
	reloadPath = "/-/reload"
)

var globalRH *requestHandler

type requestHandler struct {
	ctx                  context.Context
	vmagentAddr          string
	configUpdateInterval time.Duration
}

func Init(ctx context.Context, vmagentAddr string, configUpdateInterval time.Duration) httpserver.RequestHandler {
	globalRH = &requestHandler{
		ctx:                  ctx,
		vmagentAddr:          vmagentAddr,
		configUpdateInterval: configUpdateInterval,
	}
	go globalRH.reloadVmagent()
	return globalRH.handle
}

func (rh *requestHandler) handle(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case configPath:
		return handleConfigRequest(rh.ctx, w, r)
	default:
		return false
	}
}

func respondWithError(w http.ResponseWriter, r *http.Request, statusCode int, err error) bool {
	logger.Errorf(err.Error(), r.URL.Path)
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(err.Error()))
	return true
}

func handleConfigRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	if r.Method == http.MethodGet {
		if _, err := w.Write(models.GetConfig()); err != nil {
			return false
		}
		w.Header().Set("Content-Type", "plain/text")
		w.WriteHeader(http.StatusOK)
		logger.Infof("Sent config to vmagent")
		return true
	}
	return respondWithError(w, r, http.StatusBadRequest, fmt.Errorf("got unsupported HTTP method: %s", r.Method))
}

func (rh *requestHandler) reloadVmagent() {
	ticker := time.NewTicker(rh.configUpdateInterval)
	defer ticker.Stop()

	for range ticker.C {
		if resp, err := http.Get("http://" + rh.vmagentAddr + reloadPath); err != nil {
			logger.Errorf("error made get request: %s", err)
		} else {
			if resp.StatusCode/100 != 2 {
				scanner := bufio.NewScanner(io.LimitReader(resp.Body, 1024))
				line := ""
				if scanner.Scan() {
					line = scanner.Text()
				}
				logger.Errorf("server returned HTTP status %s (%d): %s", resp.Status, resp.StatusCode, line)
			}
		}
	}
}
