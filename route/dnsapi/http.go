package dnsapi

import (
	"fmt"
	"net/http"

	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func Router() http.Handler {
	r := chi.NewRouter()
	r.Post("/lease", createLease)
	r.Post("/renew", renewLease)
	r.Delete("/lease", deleteLease)
	return r
}

func createLease(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Servers []string `json:"servers"`
	}
	if err := httphelper.DecodeRequest(r, &req); err != nil {
		httphelper.SendError(w, httphelper.BadRequest(fmt.Sprintf("Invalid request body: %v", err)))
		return
	}
	if err := validateServers(req.Servers); err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}
	global.mu.Lock()
	err := global.set(req.Servers)
	global.mu.Unlock()
	if err != nil {
		httphelper.SendError(w, err)
		return
	}
	render.NoContent(w, r)
}

func renewLease(w http.ResponseWriter, r *http.Request) {
	global.mu.Lock()
	err := global.renew()
	global.mu.Unlock()
	if err != nil {
		httphelper.SendError(w, httphelper.Conflict(err.Error()))
		return
	}
	render.NoContent(w, r)
}

func deleteLease(w http.ResponseWriter, r *http.Request) {
	global.mu.Lock()
	err := global.release()
	global.mu.Unlock()
	if err != nil {
		httphelper.SendError(w, err)
		return
	}
	render.NoContent(w, r)
}
