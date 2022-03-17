package controllers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"prometheus-benchmark/services/vmagent-config-update/models"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	configPath = "/api/v1/config"
)

var globalRH *requestHandler

type requestHandler struct {
	ctx                  context.Context
	configUpdateInterval time.Duration
}

func Init(ctx context.Context, configUpdateInterval time.Duration) httpserver.RequestHandler {
	globalRH = &requestHandler{
		ctx:                  ctx,
		configUpdateInterval: configUpdateInterval,
	}
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
