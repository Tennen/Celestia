package control

import "github.com/chentianyu/celestia/internal/models"

func (s *Service) Specs(device models.Device) []models.DeviceControlSpec {
	return parseDeclaredControls(device)
}
