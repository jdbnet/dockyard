package engine

import (
	"context"

	"github.com/jdbnet/dockyard/internal/notify"
)

type NotificationsResponse struct {
	notify.SettingsView
	Status notify.Status `json:"status"`
}

func (e *Engine) NotificationSettings() NotificationsResponse {
	return NotificationsResponse{
		SettingsView: notify.PublicSettings(e.cfg.Notifications),
		Status:       e.notifier.Status(),
	}
}

func (e *Engine) UpdateNotifications(update notify.SettingsUpdate) (NotificationsResponse, error) {
	next := notify.MergeUpdate(e.cfg.Notifications, update)
	var err error
	next, err = notify.Normalise(next)
	if err != nil {
		return NotificationsResponse{}, err
	}
	if err := e.cfg.SaveNotifications(next); err != nil {
		return NotificationsResponse{}, err
	}
	e.cfg.Notifications = next
	e.notifier.Update(next)
	return e.NotificationSettings(), nil
}

func (e *Engine) TestNotification(ctx context.Context, req notify.TestRequest) error {
	return e.notifier.Test(ctx, e.cfg.Notifications, req)
}
