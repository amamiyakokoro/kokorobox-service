package coreapi

import (
	"net/http"
	"sync"

	corepkg "github.com/amamiyakokoro/kokorobox-service/core"
	"github.com/amamiyakokoro/kokorobox-service/route/auth"
	"github.com/amamiyakokoro/kokorobox-service/route/httphelper"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

var (
	cm     *corepkg.CoreManager
	initMu sync.Mutex
)

func initCoreManager() {
	initMu.Lock()
	defer initMu.Unlock()
	if cm == nil {
		cm = corepkg.NewCoreManager(corepkg.WithTrafficMonitorPipeSDDL(trafficMonitorPipeSDDL()))
	}
}

func Router() http.Handler {
	initCoreManager()

	r := chi.NewRouter()

	r.Get("/", coreStatus)
	r.Get("/desired", coreDesiredStatus)
	r.Get("/events", coreEvents)
	r.HandleFunc("/controller", coreControllerProxy)
	r.HandleFunc("/controller/*", coreControllerProxy)
	r.Get("/profile", coreProfile)
	r.Post("/profile", coreSaveProfile)
	r.Patch("/profile", corePatchProfile)
	r.Post("/start", coreStart)
	r.Post("/stop", coreStop)
	r.Post("/restart", coreRestart)

	return r
}

func trafficMonitorPipeSDDL() string {
	if sid, ok := auth.GetKeyManager().GetAuthorizedSID(); ok {
		return "D:PAI(A;OICI;GWGR;;;" + sid + ")(A;OICI;GWGR;;;SY)"
	}
	return ""
}

func Stop() error {
	desiredCore.stopWatching()
	if cm == nil {
		return nil
	}
	return cm.StopCore()
}

func coreDesiredStatus(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]any{"desired_state": map[bool]string{true: "running", false: "stopped"}[desiredCore.desired()]})
}

func coreStatus(w http.ResponseWriter, r *http.Request) {
	status, err := cm.GetProcessInfo()
	if err != nil {
		httphelper.SendError(w, err)
		return
	}
	render.JSON(w, r, status)
}

func coreProfile(w http.ResponseWriter, r *http.Request) {
	profile, err := corepkg.LoadLaunchProfile()
	if err != nil {
		httphelper.SendError(w, err)
		return
	}
	render.JSON(w, r, profile)
}

func coreSaveProfile(w http.ResponseWriter, r *http.Request) {
	var profile corepkg.LaunchProfile
	if err := httphelper.DecodeRequest(r, &profile); err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}

	if err := corepkg.SaveLaunchProfile(profile); err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}
	normalized, err := corepkg.LoadLaunchProfile()
	if err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}
	cm.ApplyLaunchProfile(normalized, coreLaunchOptions(r)...)

	httphelper.SendJSON(w, "success", "Core launch configuration updated")
}

func corePatchProfile(w http.ResponseWriter, r *http.Request) {
	var patch corepkg.LaunchProfilePatch
	if err := httphelper.DecodeRequest(r, &patch); err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}

	profile, err := corepkg.PatchLaunchProfile(patch)
	if err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}
	cm.ApplyLaunchProfile(profile, coreLaunchOptions(r)...)

	httphelper.SendJSON(w, "success", "Core launch configuration updated")
}

func coreStart(w http.ResponseWriter, r *http.Request) {
	profile, hasProfile, err := decodeOptionalLaunchProfile(r)
	if err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}
	if hasProfile {
		if err := corepkg.SaveLaunchProfile(*profile); err != nil {
			httphelper.SendError(w, httphelper.BadRequest(err.Error()))
			return
		}
	}

	if err := startDesiredCore(profile, coreLaunchGroup(r), coreLaunchOptions(r)...); err != nil {
		httphelper.SendError(w, err)
		return
	}

	sendCoreReady(w, r, "Core started successfully")
}

func coreStop(w http.ResponseWriter, r *http.Request) {
	if err := desiredCore.set(false, nil); err != nil {
		httphelper.SendError(w, err)
		return
	}
	if err := cm.StopCore(); err != nil {
		httphelper.SendError(w, err)
		return
	}
	httphelper.SendJSON(w, "success", "Core stopped successfully")
}

func coreRestart(w http.ResponseWriter, r *http.Request) {
	profile, hasProfile, err := decodeOptionalLaunchProfile(r)
	if err != nil {
		httphelper.SendError(w, httphelper.BadRequest(err.Error()))
		return
	}
	if hasProfile {
		if err := corepkg.SaveLaunchProfile(*profile); err != nil {
			httphelper.SendError(w, httphelper.BadRequest(err.Error()))
			return
		}
	}

	if err := desiredCore.set(true, coreLaunchGroup(r)); err != nil {
		httphelper.SendError(w, err)
		return
	}
	desiredCore.mu.Lock()
	if err := cm.RestartCoreWithProfile(profile, coreLaunchOptions(r)...); err != nil {
		desiredCore.mu.Unlock()
		httphelper.SendError(w, err)
		return
	}
	desiredCore.mu.Unlock()
	sendCoreReady(w, r, "Core restarted successfully")
}

func decodeOptionalLaunchProfile(r *http.Request) (*corepkg.LaunchProfile, bool, error) {
	var profile corepkg.LaunchProfile
	ok, err := httphelper.DecodeOptionalRequest(r, &profile)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	return new(profile), true, nil
}

func sendCoreReady(w http.ResponseWriter, r *http.Request, message string) {
	status, err := cm.GetProcessInfo()
	if err != nil {
		httphelper.SendJSON(w, "success", message)
		return
	}

	render.JSON(w, r, map[string]any{
		"status":  "success",
		"message": message,
		"core":    status,
	})
}
