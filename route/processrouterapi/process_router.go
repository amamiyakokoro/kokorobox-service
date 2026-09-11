package processrouterapi

import (
	"errors"
	"net/http"
	"sync"

	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/amamiyakokoro/kokorobox-service/processrouter"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

var (
	manager     = processrouter.NewDefaultManager()
	restoreOnce sync.Once
)

func Router() http.Handler {
	restoreOnce.Do(func() {
		if err := manager.Restore(); err != nil && !errors.Is(err, processrouter.ErrUnsupported) {
			log.Printf("恢复应用分流服务失败: %v", err)
		}
	})
	r := chi.NewRouter()
	r.Post("/start", start)
	r.Post("/stop", stop)
	r.Put("/rules", replaceRules)
	r.Get("/status", status)
	r.Post("/firewall/repair", repairFirewall)
	r.Post("/cleanup", cleanup)
	return r
}

func Stop() error {
	return manager.Close()
}

func start(w http.ResponseWriter, r *http.Request) {
	status, err := manager.Enable()
	if err != nil {
		sendManagerError(w, err)
		return
	}
	render.JSON(w, r, status)
}

func stop(w http.ResponseWriter, r *http.Request) {
	if err := manager.Disable(); err != nil {
		httphelper.SendError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func replaceRules(w http.ResponseWriter, r *http.Request) {
	var request processrouter.RulesRequest
	if err := httphelper.DecodeRequest(r, &request); err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}
	status, err := manager.ReplaceRules(request)
	if err != nil {
		sendManagerError(w, err)
		return
	}
	render.JSON(w, r, status)
}

func status(w http.ResponseWriter, r *http.Request) {
	manager.RenewLease()
	render.JSON(w, r, manager.Status())
}

func repairFirewall(w http.ResponseWriter, r *http.Request) {
	if err := manager.RepairFirewall(); err != nil {
		sendManagerError(w, err)
		return
	}
	render.JSON(w, r, manager.Status())
}

func cleanup(w http.ResponseWriter, r *http.Request) {
	if err := manager.Cleanup(); err != nil {
		httphelper.SendError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func sendManagerError(w http.ResponseWriter, err error) {
	var validationError *processrouter.ValidationError
	if errors.As(err, &validationError) {
		httphelper.SendError(w, httphelper.BadRequest(validationError.Error()))
		return
	}
	if errors.Is(err, processrouter.ErrUnsupported) {
		httphelper.SendError(w, httphelper.NewError(http.StatusNotImplemented, err.Error()))
		return
	}
	httphelper.SendError(w, err)
}
