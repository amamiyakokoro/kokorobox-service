package processrouter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const commandTimeout = 5 * time.Second

type routerEvent struct {
	Version int    `json:"version"`
	Event   string `json:"event"`
	Message string `json:"message,omitempty"`
}

type nativeProcess interface {
	PID() int
	Alive() bool
	Send([]byte) error
	Events() <-chan routerEvent
	Stop(context.Context) error
}

func sendRules(process nativeProcess, command routerCommand) error {
	payload, err := json.Marshal(command)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	if err := process.Send(payload); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return errors.New("process router command timed out")
		case event, ok := <-process.Events():
			if !ok {
				return errors.New("process router stopped before acknowledging rules")
			}
			if event.Version != ProtocolVersion {
				return fmt.Errorf("unsupported native router protocol version: %d", event.Version)
			}
			switch event.Event {
			case "rules_replaced":
				return nil
			case "error":
				if event.Message == "" {
					event.Message = "native router rejected rules"
				}
				return errors.New(event.Message)
			}
		}
	}
}
