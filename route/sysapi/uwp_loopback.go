package sysapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
)

type uwpLoopbackRequest struct {
	ID      string `json:"id"`
	Enabled *bool  `json:"enabled"`
}

func setUwpLoopback(w http.ResponseWriter, r *http.Request) {
	var req uwpLoopbackRequest
	if err := httphelper.DecodeRequest(r, &req); err != nil {
		httphelper.SendError(w, httphelper.BadRequest(fmt.Sprintf("Invalid request body: %v", err)))
		return
	}
	if len(req.ID) < 16 || len(req.ID) > 136 || len(req.ID)%2 != 0 || strings.Trim(req.ID, "0123456789abcdefABCDEF") != "" || req.Enabled == nil {
		httphelper.SendError(w, httphelper.BadRequest("Invalid UWP app container or enabled state"))
		return
	}
	if err := updateUwpLoopback(req.ID, *req.Enabled); err != nil {
		httphelper.SendError(w, err)
		return
	}
	httphelper.SendJSON(w, "success", "UWP loopback exemption updated")
}
