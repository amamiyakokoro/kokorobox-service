package coreapi

import (
	corepkg "github.com/amamiyakokoro/kokorobox-service/core"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	"github.com/go-chi/render"
	"net/http"
)

func validateProfile(w http.ResponseWriter, r *http.Request) {
	var request corepkg.ProfileValidationRequest
	if err := httphelper.DecodeRequest(r, &request); err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}
	result, err := cm.ValidateProfile(r.Context(), request)
	if err != nil {
		httphelper.SendError(w, err)
		return
	}
	render.JSON(w, r, result)
}
