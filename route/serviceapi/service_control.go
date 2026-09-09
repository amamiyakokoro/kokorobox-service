package serviceapi

import (
	"net/http"
	"time"

	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	appservice "github.com/amamiyakokoro/kokorobox-service/service"

	"github.com/go-chi/chi/v5"
)

var serviceController appservice.Controller

func Router() http.Handler {
	r := chi.NewRouter()

	r.Post("/stop", serviceStop)
	r.Post("/restart", serviceRestart)

	return r
}

func serviceStop(w http.ResponseWriter, r *http.Request) {
	controlServiceAsync(w, "stop", "Service is stopping...", func() error { return serviceController.Stop() })
}

func serviceRestart(w http.ResponseWriter, r *http.Request) {
	controlServiceAsync(w, "restart", "Service is restarting...", func() error { return serviceController.Restart() })
}

func controlServiceAsync(w http.ResponseWriter, action string, message string, fn func() error) {
	status, err := serviceController.Status()
	if err != nil {
		httphelper.SendError(w, err)
		return
	}

	if action == "stop" && status == appservice.StatusStopped {
		httphelper.SendJSON(w, "success", "Service is stopped")
		return
	}

	if action == "restart" && status == appservice.StatusStopped {
		httphelper.SendError(w, httphelper.Conflict("Service is not running"))
		return
	}

	httphelper.SendJSONWithStatus(w, http.StatusAccepted, "success", message)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	go func() {
		time.Sleep(200 * time.Millisecond)
		if err := fn(); err != nil {
			log.Printf("%s 服务失败: %v", action, err)
		}
	}()
}
