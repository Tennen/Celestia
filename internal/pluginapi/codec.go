package pluginapi

import (
	"encoding/json"

	"google.golang.org/protobuf/types/known/structpb"
)

func EncodeStruct(v any) (*structpb.Struct, error) {
	if v == nil {
		return structpb.NewStruct(map[string]any{})
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	data := map[string]any{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return structpb.NewStruct(data)
}

func DecodeStruct(in *structpb.Struct, out any) error {
	if in == nil {
		raw, err := json.Marshal(map[string]any{})
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, out)
	}
	raw, err := json.Marshal(in.AsMap())
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func EncodeList(v any) (*structpb.ListValue, error) {
	if v == nil {
		return &structpb.ListValue{}, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var items []any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	return structpb.NewList(items)
}

func DecodeList(in *structpb.ListValue, out any) error {
	if in == nil {
		raw, err := json.Marshal([]any{})
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, out)
	}
	raw, err := json.Marshal(in.AsSlice())
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

