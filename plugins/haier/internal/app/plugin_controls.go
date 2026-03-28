package app

import "github.com/chentianyu/celestia/internal/models"

func buildControlSpecs(capabilitySet map[string]bool) []models.DeviceControlSpec {
	specs := make([]models.DeviceControlSpec, 0, 3)
	appendAction := func(id, label, action string) {
		if !capabilitySet[action] {
			return
		}
		specs = append(specs, models.DeviceControlSpec{
			ID:    id,
			Kind:  models.DeviceControlKindAction,
			Label: label,
			Command: &models.DeviceControlCommand{
				Action: action,
			},
		})
	}
	appendAction("start", "Start", "start")
	appendAction("pause", "Pause", "pause")
	appendAction("resume", "Resume", "resume")
	return specs
}
