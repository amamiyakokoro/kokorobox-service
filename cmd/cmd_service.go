package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/amamiyakokoro/kokorobox-service/i18n"
	"github.com/amamiyakokoro/kokorobox-service/log"
	"github.com/amamiyakokoro/kokorobox-service/processrouter"
	"github.com/amamiyakokoro/kokorobox-service/route"
	appservice "github.com/amamiyakokoro/kokorobox-service/service"

	kservice "github.com/kardianos/service"
	"github.com/spf13/cobra"
)

type Program struct {
	listen string
}

type serviceCommandStatus struct {
	Action  string `json:"action,omitempty"`
	State   string `json:"state,omitempty"`
	Success bool   `json:"success"`
	Changed bool   `json:"changed,omitempty"`
	Error   string `json:"error,omitempty"`
}

func (p *Program) Start(s kservice.Service) error {
	go p.run()
	return nil
}

func (p *Program) run() {
	logFile, err := log.InitLogging()
	if err != nil {
		log.Printf("Failed to initialize logging: %v\n", err)
	}
	if logFile != nil {
		defer logFile.Close()
	}
	log.Println("Service is starting...")

	if err := route.Start(p.listen); err != nil {
		log.Fatal(err)
	}
}

func (p *Program) Stop(s kservice.Service) error {
	log.Println("Service is stopping...")
	if err := route.Stop(); err != nil {
		log.Printf("Service stop cleanup failed: %v", err)
		return err
	}
	log.Println("Service is stopped")
	return nil
}

func outputServiceCommandResult(message string, status serviceCommandStatus) error {
	status.Success = true
	log.S().Infow(message, "status", status)
	return nil
}

func outputServiceCommandError(action, message string, err error) error {
	if err == nil {
		return nil
	}
	log.S().Errorw(message, "status", serviceCommandStatus{
		Action:  action,
		State:   serviceErrorState(err),
		Success: false,
		Error:   err.Error(),
	})
	return newReportedError(err)
}

func serviceErrorState(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, kservice.ErrNotInstalled) || strings.Contains(strings.ToLower(err.Error()), "service is not installed") {
		return "not-installed"
	}
	return ""
}

func serviceStatusMessage(state string) string {
	switch state {
	case "running":
		return i18n.DefaultText("Service status: running")
	case "stopped":
		return i18n.DefaultText("Service status: stopped")
	default:
		return i18n.DefaultText("Service status: unknown")
	}
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: i18n.DefaultText("Install KokoroBox Service"),
	RunE: func(cmd *cobra.Command, args []string) error {
		listenAddr := listen
		if listenAddr == "" {
			listenAddr = defaultAddr
		}
		executable, err := os.Executable()
		if err != nil {
			return outputServiceCommandError("install", "Failed to resolve service executable", err)
		}
		executable, err = prepareServiceInstallExecutable(executable)
		if err != nil {
			return outputServiceCommandError("install", "Failed to prepare service executable", err)
		}

		prg := &Program{listen: listenAddr}
		s, err := appservice.New(prg, executable)
		if err != nil {
			return outputServiceCommandError("install", "Failed to create service", err)
		}

		if err := installServiceRegistration(s); err != nil {
			return outputServiceCommandError("install", "Failed to install service", err)
		}
		if err := s.Start(); err != nil {
			return outputServiceCommandError("install", "Failed to start service", err)
		}
		return outputServiceCommandResult("Service installed successfully", serviceCommandStatus{Action: "install", State: "running"})
	},
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: i18n.DefaultText("Uninstall KokoroBox Service"),
	RunE: func(cmd *cobra.Command, args []string) error {
		listenAddr := listen
		if listenAddr == "" {
			listenAddr = defaultAddr
		}
		prg := &Program{listen: listenAddr}
		s, err := appservice.New(prg, "")
		if err != nil {
			return outputServiceCommandError("uninstall", "Failed to create service", err)
		}

		if err := s.Stop(); err != nil {
			return outputServiceCommandError("uninstall", "Failed to stop service", err)
		}
		if err := processrouter.RemoveFirewallRules(); err != nil {
			return outputServiceCommandError("uninstall", "Failed to remove application-routing firewall rules", err)
		}
		if err := s.Uninstall(); err != nil {
			return outputServiceCommandError("uninstall", "Failed to uninstall service", err)
		}
		return outputServiceCommandResult("Service uninstalled successfully", serviceCommandStatus{Action: "uninstall", State: "not-installed"})
	},
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: i18n.DefaultText("Start KokoroBox Service"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := (appservice.Controller{}).Start(); err != nil {
			return outputServiceCommandError("start", "Failed to start service", err)
		}
		return outputServiceCommandResult("Service started successfully", serviceCommandStatus{Action: "start", State: "running"})
	},
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: i18n.DefaultText("Stop KokoroBox Service"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := (appservice.Controller{}).Stop(); err != nil {
			return outputServiceCommandError("stop", "Failed to stop service", err)
		}
		return outputServiceCommandResult("Service stopped successfully", serviceCommandStatus{Action: "stop", State: "stopped"})
	},
}

var serviceRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: i18n.DefaultText("Restart KokoroBox Service"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := (appservice.Controller{}).Restart(); err != nil {
			return outputServiceCommandError("restart", "Failed to restart service", err)
		}
		return outputServiceCommandResult("Service restarted successfully", serviceCommandStatus{Action: "restart", State: "running"})
	},
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: i18n.DefaultText("Show KokoroBox Service status"),
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := (appservice.Controller{}).Status()
		if err != nil {
			return outputServiceCommandError("status", "Failed to query service status", err)
		}

		state := string(status)
		return outputServiceCommandResult(serviceStatusMessage(state), serviceCommandStatus{Action: "status", State: state})
	},
}

var serviceRunCmd = &cobra.Command{
	Use:   "run",
	Short: i18n.DefaultText("Run KokoroBox Service"),
	Run: func(cmd *cobra.Command, args []string) {
		if err := ensureServiceRuntimeExecutable(); err != nil {
			log.Fatal(err)
		}

		listenAddr := listen
		if listenAddr == "" {
			listenAddr = defaultAddr
		}
		prg := &Program{listen: listenAddr}
		s, err := appservice.New(prg, "")
		if err != nil {
			log.Fatal(err)
		}

		if err := s.Run(); err != nil {
			log.Fatal(err)
		}
	},
}

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: i18n.DefaultText("Manage KokoroBox Service"),
}

var serviceInitCmd = &cobra.Command{
	Use:   "init",
	Short: i18n.DefaultText("Initialize the service with a public key"),
	RunE: func(cmd *cobra.Command, args []string) error {
		publicKey := cmd.Flag("public-key").Value.String()
		authorizedSID := cmd.Flag("authorized-sid").Value.String()
		authorizedUID, _ := cmd.Flags().GetUint32("authorized-uid")
		if publicKey == "" {
			return outputServiceCommandError("init", "Error: a public key must be provided with --public-key", errors.New("A public key must be provided with --public-key"))
		}
		if authorizedSID == "" && !cmd.Flags().Changed("authorized-uid") {
			return outputServiceCommandError("init", "Error: an authorized user identity must be bound with --authorized-sid or --authorized-uid", errors.New("An authorized user identity must be bound with --authorized-sid or --authorized-uid"))
		}
		dataDir, err := route.GetServiceDataDir()
		if err != nil {
			return outputServiceCommandError("init", "Failed to prepare service data directory", err)
		}
		keyDir := filepath.Join(dataDir, "keys")

		_ = route.InitKeyManager(keyDir)

		km := route.GetKeyManager()
		keyChanged, err := km.SetPublicKey(publicKey)
		if err != nil {
			return outputServiceCommandError("init", "Failed to set public key", err)
		}

		principalChanged := false
		switch {
		case authorizedSID != "":
			principalChanged, err = km.SetAuthorizedSID(authorizedSID)
			if err != nil {
				return outputServiceCommandError("init", "Failed to set authorized SID", err)
			}
		case cmd.Flags().Changed("authorized-uid"):
			principalChanged, err = km.SetAuthorizedUID(authorizedUID)
			if err != nil {
				return outputServiceCommandError("init", "Failed to set authorized UID", err)
			}
		}

		changed := keyChanged || principalChanged
		if changed {
			_ = outputServiceCommandResult("Service initialized; authentication configuration updated", serviceCommandStatus{Action: "init", Changed: true})
		} else {
			_ = outputServiceCommandResult("Service initialized; authentication configuration unchanged", serviceCommandStatus{Action: "init"})
		}

		controller := appservice.Controller{}
		status, err := controller.Status()
		if err != nil {
			return outputServiceCommandError("status", "Failed to query service status; if the service is running, run 'restart' manually", err)
		}

		state := string(status)
		if status == appservice.StatusRunning {
			if !changed {
				return outputServiceCommandResult("Service is already running; configuration is unchanged and no restart is needed", serviceCommandStatus{Action: "init", State: state})
			}
			log.S().Infow("Restarting service...", "status", serviceCommandStatus{Action: "restart", State: state, Success: true})
			if err := controller.Restart(); err != nil {
				return outputServiceCommandError("restart", "Failed to restart service; run 'kokorobox-service service restart' manually", err)
			}
			return outputServiceCommandResult("Service restarted successfully", serviceCommandStatus{Action: "restart", State: "running"})
		}

		return outputServiceCommandResult("Service is not running; configuration will apply on the next start", serviceCommandStatus{Action: "init", State: state, Changed: changed})
	},
}

func init() {
	serviceCmd.AddCommand(serviceInitCmd)
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceRestartCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	serviceCmd.AddCommand(serviceRunCmd)

	serviceInitCmd.Flags().StringP("public-key", "k", "", i18n.DefaultText("Client public key"))
	serviceInitCmd.Flags().String("authorized-sid", "", i18n.DefaultText("Windows SID allowed to access the service"))
	serviceInitCmd.Flags().Uint32("authorized-uid", 0, i18n.DefaultText("Unix UID allowed to access the service"))
}
