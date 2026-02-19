package dprint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

type rawFormatConfigEnvelope struct {
	Plugin map[string]json.RawMessage `json:"plugin"`
	Global map[string]json.RawMessage `json:"global"`
}

func parseRawFormatConfig(data []byte) (RawFormatConfig, error) {
	var envelope rawFormatConfigEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return RawFormatConfig{}, err
	}

	pluginConfig, err := decodeRawObject(envelope.Plugin)
	if err != nil {
		return RawFormatConfig{}, fmt.Errorf("failed to decode plugin config: %w", err)
	}

	globalConfig, err := decodeRawObject(envelope.Global)
	if err != nil {
		return RawFormatConfig{}, fmt.Errorf("failed to decode global config: %w", err)
	}

	return RawFormatConfig{
		Plugin: pluginConfig,
		Global: GlobalConfiguration(globalConfig),
	}, nil
}

func decodeRawObject(rawMap map[string]json.RawMessage) (map[string]any, error) {
	if rawMap == nil {
		return nil, nil
	}

	result := make(map[string]any, len(rawMap))
	for key, rawValue := range rawMap {
		value, err := decodeRawValue(rawValue)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", key, err)
		}
		result[key] = value
	}
	return result, nil
}

func decodeRawArray(rawArray []json.RawMessage) ([]any, error) {
	result := make([]any, len(rawArray))
	for i, rawValue := range rawArray {
		value, err := decodeRawValue(rawValue)
		if err != nil {
			return nil, fmt.Errorf("index %d: %w", i, err)
		}
		result[i] = value
	}
	return result, nil
}

func decodeRawValue(raw json.RawMessage) (any, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}

	switch trimmed[0] {
	case '{':
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &nested); err != nil {
			return nil, err
		}
		return decodeRawObject(nested)
	case '[':
		var nested []json.RawMessage
		if err := json.Unmarshal(trimmed, &nested); err != nil {
			return nil, err
		}
		return decodeRawArray(nested)
	case '"':
		var text string
		if err := json.Unmarshal(trimmed, &text); err != nil {
			return nil, err
		}
		return text, nil
	case 't', 'f':
		var boolValue bool
		if err := json.Unmarshal(trimmed, &boolValue); err != nil {
			return nil, err
		}
		return boolValue, nil
	case 'n':
		return nil, nil
	default:
		numberText := string(trimmed)
		if intValue, err := strconv.ParseInt(numberText, 10, 64); err == nil {
			return intValue, nil
		}
		if floatValue, err := strconv.ParseFloat(numberText, 64); err == nil {
			return floatValue, nil
		}
		return nil, fmt.Errorf("unsupported raw value: %q", numberText)
	}
}
