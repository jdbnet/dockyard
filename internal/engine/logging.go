package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/jdbnet/dockyard/internal/logging"
)

func (e *Engine) LoggingSettings() logging.SettingsView {
	return logging.PublicSettings(e.shipper.Settings())
}

func (e *Engine) UpdateLogging(update logging.SettingsUpdate) (logging.SettingsView, error) {
	next := logging.MergeUpdate(e.shipper.Settings(), update)
	var err error
	next, err = logging.Normalise(next)
	if err != nil {
		return logging.SettingsView{}, err
	}
	if err := e.cfg.SaveLogging(next); err != nil {
		return logging.SettingsView{}, err
	}
	e.cfg.Logging = next
	e.shipper.Update(next)
	return logging.PublicSettings(next), nil
}

func (e *Engine) LoggingStatus() logging.Status {
	return e.shipper.Status()
}

func (e *Engine) TestLogging(ctx context.Context, update logging.SettingsUpdate) error {
	draft := logging.MergeUpdate(e.shipper.Settings(), update)
	var err error
	draft, err = logging.Normalise(draft)
	if err != nil {
		return err
	}
	if strings.TrimSpace(draft.URL) == "" {
		return fmt.Errorf("destination URL is required")
	}
	return e.shipper.Test(ctx, draft)
}
