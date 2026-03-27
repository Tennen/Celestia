package app

import (
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (p *Plugin) emitSnapshotEventLocked(snapshot deviceSnapshot) {
	if snapshot.LatestEvent == nil {
		return
	}
	if snapshot.LatestEvent.Key == "" {
		return
	}
	if previous := p.eventKeys[snapshot.Device.ID]; previous == snapshot.LatestEvent.Key {
		return
	}
	p.eventKeys[snapshot.Device.ID] = snapshot.LatestEvent.Key

	ts := snapshot.LatestEvent.TS
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	p.emitLocked(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceOccurred,
		PluginID: "petkit",
		DeviceID: snapshot.Device.ID,
		TS:       ts,
		Payload:  snapshot.LatestEvent.Payload,
	})
}
