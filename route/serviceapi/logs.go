package serviceapi

import (
	"net/http"

	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	"github.com/go-chi/render"
)

func serviceLogs(w http.ResponseWriter, r *http.Request) {
	snapshot, err := log.ReadSnapshot()
	if err != nil {
		httphelper.SendError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	render.JSON(w, r, snapshot)
}
