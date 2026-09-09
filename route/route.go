package route

import (
	"github.com/amamiyakokoro/kokorobox-service/route/auth"
	"github.com/amamiyakokoro/kokorobox-service/route/coreapi"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"
	"github.com/amamiyakokoro/kokorobox-service/route/processrouterapi"
	"github.com/amamiyakokoro/kokorobox-service/route/serviceapi"
	"github.com/amamiyakokoro/kokorobox-service/route/sysapi"
	"github.com/amamiyakokoro/kokorobox-service/route/sysproxyapi"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func router() *chi.Mux {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(httphelper.LocaleMiddleware)
	r.Use(httphelper.RequestLogger)

	r.Group(func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			httphelper.SendJSON(w, "success", "pong")
		})
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.AuthMiddleware)
		r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			httphelper.SendJSON(w, "success", "auth success")
		})
		r.Mount("/service", serviceapi.Router())
		r.Mount("/sysproxy", sysproxyapi.Router())
		r.Mount("/core", coreapi.Router())
		r.Mount("/process-router", processrouterapi.Router())
		r.Mount("/sys", sysapi.Router())
	})
	return r
}
