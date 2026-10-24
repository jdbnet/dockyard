package engine

import "github.com/jdbnet/dockyard/internal/notify"

func toNotifyReport(report UpdatesReport) notify.UpdateReport {
	pending := make([]notify.PendingRow, len(report.Pending))
	for i, row := range report.Pending {
		pending[i] = notify.PendingRow{
			ContainerID:  row.ContainerID,
			ShortID:      row.ShortID,
			Name:         row.Name,
			Image:        row.Image,
			LocalDigest:  row.LocalDigest,
			RemoteDigest: row.RemoteDigest,
		}
	}
	return notify.UpdateReport{
		ScannedAt: report.ScannedAt,
		Pending:   pending,
	}
}

func (e *Engine) notifyReport() notify.UpdateReport {
	return toNotifyReport(e.Updates())
}
