package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/plugins/xiaomi/internal/mapper"
	"github.com/google/uuid"
)

func (p *Plugin) setPower(ctx context.Context, runtime *deviceRuntime, on bool) error {
	if runtime.mapping.Power != nil {
		return p.setPropertyCommand(ctx, runtime, runtime.mapping.Power, on)
	}
	if runtime.mapping.Kind == models.DeviceKindAquarium {
		var errs []string
		for _, ref := range []*mapper.PropertyRef{runtime.mapping.PumpPower, runtime.mapping.LightPower} {
			if ref == nil {
				continue
			}
			if err := p.setPropertyCommand(ctx, runtime, ref, on); err != nil {
				errs = append(errs, err.Error())
			}
		}
		if len(errs) > 0 {
			return errors.New(strings.Join(errs, "; "))
		}
		return nil
	}
	return errors.New("power unsupported")
}

func (p *Plugin) setToggle(ctx context.Context, runtime *deviceRuntime, toggleID string, on bool) error {
	for _, item := range runtime.mapping.ToggleChannels {
		if item.ID == toggleID && item.Ref != nil {
			return p.setPropertyCommand(ctx, runtime, item.Ref, on)
		}
	}
	if toggleID == "power" || toggleID == "" {
		return p.setPower(ctx, runtime, on)
	}
	return fmt.Errorf("toggle %q unsupported", toggleID)
}

func (p *Plugin) setPropertyCommand(ctx context.Context, runtime *deviceRuntime, ref *mapper.PropertyRef, raw any) error {
	if ref == nil {
		return errors.New("capability unsupported")
	}
	value, err := encodePropertyValue(ref.Property, raw)
	if err != nil {
		return err
	}
	results, err := runtime.account.client.SetProps(ctx, []map[string]any{{
		"did":   runtime.raw.DID,
		"siid":  ref.ServiceIID,
		"piid":  ref.Property.IID,
		"value": value,
	}})
	if err != nil {
		return err
	}
	for _, result := range results {
		if code := intParam(result["code"]); code != 0 {
			return fmt.Errorf("xiaomi command rejected: code=%d", code)
		}
	}
	return nil
}

func (p *Plugin) pushVoiceMessage(ctx context.Context, runtime *deviceRuntime, params map[string]any) error {
	message := strings.TrimSpace(stringParam(params["message"]))
	if message == "" {
		return errors.New("message is required")
	}
	if volume, ok := params["volume"]; ok && runtime.mapping.Volume != nil {
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.Volume, volume); err != nil {
			return err
		}
	}
	switch {
	case runtime.mapping.NotifyAction != nil:
		inputs := buildActionInputs(runtime.mapping.NotifyAction, message, params)
		result, err := runtime.account.client.Action(ctx, runtime.raw.DID, runtime.mapping.NotifyAction.ServiceIID, runtime.mapping.NotifyAction.Action.IID, inputs)
		if err != nil {
			return err
		}
		if code := intParam(result["code"]); code != 0 {
			return fmt.Errorf("xiaomi notify action rejected: code=%d", code)
		}
	case runtime.mapping.Text != nil:
		if err := p.setPropertyCommand(ctx, runtime, runtime.mapping.Text, message); err != nil {
			return err
		}
	default:
		return errors.New("voice_push unsupported")
	}
	p.emit(models.Event{
		ID:       uuid.NewString(),
		Type:     models.EventDeviceOccurred,
		PluginID: "xiaomi",
		DeviceID: runtime.device.ID,
		TS:       time.Now().UTC(),
		Payload: map[string]any{
			"event":   "speaker.text_sent",
			"message": message,
		},
	})
	return nil
}

func buildActionInputs(action *mapper.ActionRef, message string, params map[string]any) []any {
	inputs := make([]any, 0, len(action.Inputs))
	usedMessage := false
	for _, input := range action.Inputs {
		switch input.Format {
		case "string":
			if !usedMessage {
				inputs = append(inputs, message)
				usedMessage = true
			} else {
				inputs = append(inputs, message)
			}
		case "bool":
			inputs = append(inputs, boolParam(params, "on", false))
		default:
			if _, ok := params["volume"]; ok {
				inputs = append(inputs, intParam(params["volume"]))
			} else {
				inputs = append(inputs, 0)
			}
		}
	}
	return inputs
}
