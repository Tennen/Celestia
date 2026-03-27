package mapper

import "slices"

func (m DeviceMapping) capabilities() []string {
	var out []string
	appendOnce := func(value string) {
		if value == "" || slices.Contains(out, value) {
			return
		}
		out = append(out, value)
	}
	if m.Power != nil {
		appendOnce("power")
	}
	if m.Brightness != nil {
		appendOnce("brightness")
	}
	if m.ColorTemp != nil {
		appendOnce("color_temp")
	}
	if m.TargetTemperature != nil {
		appendOnce("target_temperature")
	}
	if m.Mode != nil {
		appendOnce("mode")
	}
	if m.FanSpeed != nil {
		appendOnce("fan_speed")
	}
	if m.Temperature != nil {
		appendOnce("temperature")
	}
	if m.Humidity != nil {
		appendOnce("humidity")
	}
	if m.PumpPower != nil {
		appendOnce("pump_power")
	}
	if m.LightPower != nil {
		appendOnce("light_power")
	}
	if m.LightBrightness != nil {
		appendOnce("light_brightness")
	}
	if m.LightMode != nil {
		appendOnce("light_mode")
	}
	if m.WaterTemperature != nil {
		appendOnce("water_temperature")
	}
	if m.FilterLife != nil {
		appendOnce("filter_life")
	}
	if m.Volume != nil {
		appendOnce("volume")
	}
	if m.Mute != nil {
		appendOnce("mute")
	}
	if m.NotifyAction != nil || m.Text != nil {
		appendOnce("voice_push")
	}
	if len(m.ToggleChannels) > 1 {
		appendOnce("toggle_channels")
	}
	return out
}
