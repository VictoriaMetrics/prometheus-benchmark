package controllers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const (
	configPath = "/api/v1/config"
)

var globalRH *requestHandler

type requestHandler struct {
	ctx context.Context
}

func Init(ctx context.Context) httpserver.RequestHandler {
	globalRH = &requestHandler{
		ctx: ctx,
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

		// data, err := models.GetAlertDataFromRequest(r.Body)
		// if err != nil {
		// 	return respondWithError(w, r, http.StatusBadRequest, fmt.Errorf("error decoding request from alert manager: %s", err))
		// }
		//
		// request, err := models.PrepareLokiRequest(data)
		// if err != nil {
		// 	return respondWithError(w, r, http.StatusInternalServerError, fmt.Errorf("error prepare loki request: %s", err))
		// }
		//
		// err = sender.Send(request)
		// if err != nil {
		// 	return respondWithError(w, r, http.StatusInternalServerError, fmt.Errorf("error send data to loki: %s", err))
		// }
		w.WriteHeader(http.StatusOK)
		logger.Infof("Sent information to loki")
		return true
	}
	return respondWithError(w, r, http.StatusBadRequest, fmt.Errorf("got unsupported HTTP method: %s", r.Method))
}
