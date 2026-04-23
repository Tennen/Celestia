package control

import "github.com/chentianyu/celestia/internal/models"

type HomeFilter struct {
	PluginID string
	Kind     string
	Query    string
}

type HomeDevice struct {
	ID       string
	Name     string
	Aliases  []string
	Commands []HomeCommand
}

type HomeCommand struct {
	Name     string
	Aliases  []string
	Action   string
	Params   []HomeCommandParam
	Defaults map[string]any
}

type HomeCommandParam struct {
	Name     string
	Type     models.DeviceCommandParamType
	Required bool
	Default  any
	Options  []models.DeviceControlOption
	Min      *float64
	Max      *float64
	Step     *float64
	Unit     string
}

type HomeRequest struct {
	Target     string
	DeviceID   string
	DeviceName string
	Command    string
	Action     string
	Actor      string
	Params     map[string]any
	Values     []string
}

type HomeResult struct {
	Device   HomeResolvedDevice
	Command  HomeResolvedCommand
	Decision models.PolicyDecision
	Result   models.CommandResponse
}

type HomeResolvedDevice struct {
	ID   string
	Name string
}

type HomeResolvedCommand struct {
	Name   string
	Action string
	Target string
	Params map[string]any
}

type HomeResolveMatch struct {
	DeviceID   string
	DeviceName string
	Room       string
	Command    string
	Action     string
	Target     string
}

type homeDeviceCatalog struct {
	device   HomeDevice
	model    models.Device
	commands []homeResolvedCommand
}

type homeResolvedCommand struct {
	view       HomeCommand
	kind       models.DeviceControlKind
	command    *models.DeviceControlCommand
	onCommand  *models.DeviceControlCommand
	offCommand *models.DeviceControlCommand
}

type homeResolvedTarget struct {
	catalog homeDeviceCatalog
	command homeResolvedCommand
}
