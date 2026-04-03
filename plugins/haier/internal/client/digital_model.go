package client

import "strings"

type DigitalModelValueOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type DigitalModelAttribute struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description,omitempty"`
	Value       string                    `json:"value,omitempty"`
	Readable    bool                      `json:"readable,omitempty"`
	Writable    bool                      `json:"writable,omitempty"`
	Options     []DigitalModelValueOption `json:"options,omitempty"`
}

type DigitalModel struct {
	Attributes []DigitalModelAttribute `json:"attributes"`
}

func (m DigitalModel) Values() map[string]string {
	values := make(map[string]string, len(m.Attributes))
	for _, attribute := range m.Attributes {
		name := strings.TrimSpace(attribute.Name)
		if name == "" {
			continue
		}
		values[name] = attribute.Value
	}
	return values
}
