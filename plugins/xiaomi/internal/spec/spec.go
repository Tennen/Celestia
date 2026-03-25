package spec

import (
	"strings"
)

type Instance struct {
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Services    []Service `json:"services"`
}

type Service struct {
	IID         int        `json:"iid"`
	Type        string     `json:"type"`
	Description string     `json:"description"`
	Properties  []Property `json:"properties"`
	Actions     []Action   `json:"actions"`
}

type Property struct {
	IID         int         `json:"iid"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Format      string      `json:"format"`
	Access      []string    `json:"access"`
	Unit        string      `json:"unit"`
	ValueList   []ValueItem `json:"value-list"`
	ValueRange  []float64   `json:"value-range"`
}

type ValueItem struct {
	Value       int    `json:"value"`
	Description string `json:"description"`
}

type Action struct {
	IID         int    `json:"iid"`
	Type        string `json:"type"`
	Description string `json:"description"`
	In          []int  `json:"in"`
	Out         []int  `json:"out"`
}

func DeviceType(urn string) string {
	parts := strings.Split(urn, ":")
	if len(parts) > 3 {
		return strings.ToLower(parts[3])
	}
	return normalize(urn)
}

func ServiceName(service Service) string {
	return nameFromURN(service.Type, service.Description)
}

func PropertyName(prop Property) string {
	return nameFromURN(prop.Type, prop.Description)
}

func ActionName(action Action) string {
	return nameFromURN(action.Type, action.Description)
}

func (p Property) Readable() bool {
	return hasAccess(p.Access, "read")
}

func (p Property) Writable() bool {
	return hasAccess(p.Access, "write")
}

func (p Property) Notifiable() bool {
	return hasAccess(p.Access, "notify")
}

func (p Property) RangeBounds() (min, max, step float64, ok bool) {
	if len(p.ValueRange) != 3 {
		return 0, 0, 0, false
	}
	return p.ValueRange[0], p.ValueRange[1], p.ValueRange[2], true
}

func (p Property) EnumDescription(value int) (string, bool) {
	for _, item := range p.ValueList {
		if item.Value == value {
			return normalize(item.Description), true
		}
	}
	return "", false
}

func (p Property) EnumValue(input string) (int, bool) {
	needle := normalize(input)
	for _, item := range p.ValueList {
		if normalize(item.Description) == needle {
			return item.Value, true
		}
	}
	return 0, false
}

func nameFromURN(urn, fallback string) string {
	parts := strings.Split(urn, ":")
	if len(parts) > 3 {
		return normalize(parts[3])
	}
	return normalize(fallback)
}

func hasAccess(access []string, target string) bool {
	target = strings.ToLower(target)
	for _, item := range access {
		if strings.ToLower(item) == target {
			return true
		}
	}
	return false
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "_", "-")
	return value
}
