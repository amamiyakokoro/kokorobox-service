package log

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/amamiyakokoro/kokorobox-service/i18n"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLogsStayEnglishAcrossDisplayLocales(t *testing.T) {
	previous := i18n.Default()
	t.Cleanup(func() { i18n.SetDefault(string(previous)) })
	for _, locale := range []string{"en", "zh-CN", "zh-TW"} {
		t.Run(locale, func(t *testing.T) {
			i18n.SetDefault(locale)
			var output bytes.Buffer
			logger := newLogger(zapcore.AddSync(&output))
			const userPath = `C:\使用者\應用程式.exe`
			logger.With(zap.String("path", userPath)).Info("Core started successfully",
				zap.Error(fmt.Errorf("Failed to open core log file: %w", errors.New("access denied"))))
			logger.Sugar().Infof("Native process router: %s", "diagnostic engine=[PID] Owner unresolved after retry")
			decoder := json.NewDecoder(&output)
			var entry map[string]any
			if err := decoder.Decode(&entry); err != nil {
				t.Fatal(err)
			}
			if entry["msg"] != "Core started successfully" || entry["path"] != userPath ||
				entry["error"] != "Failed to open core log file: access denied" {
				t.Fatalf("message or structured data changed for %s: %+v", locale, entry)
			}
			if err := decoder.Decode(&entry); err != nil {
				t.Fatal(err)
			}
			if entry["msg"] != "Native process router: diagnostic engine=[PID] Owner unresolved after retry" {
				t.Fatalf("formatted diagnostic changed for %s: %+v", locale, entry)
			}
		})
	}
}
