package dprint

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// CoerceUInt32 attempts to convert common JSON-like values into uint32.
func CoerceUInt32(value any) (uint32, bool) {
	switch value := value.(type) {
	case uint32:
		return value, true
	case uint16:
		return uint32(value), true
	case uint8:
		return uint32(value), true
	case uint64:
		if value > math.MaxUint32 {
			return 0, false
		}
		return uint32(value), true
	case uint:
		if uint64(value) > math.MaxUint32 {
			return 0, false
		}
		return uint32(value), true
	case int8:
		if value < 0 {
			return 0, false
		}
		return uint32(value), true
	case int16:
		if value < 0 {
			return 0, false
		}
		return uint32(value), true
	case int32:
		if value < 0 {
			return 0, false
		}
		return uint32(value), true
	case int:
		if value < 0 {
			return 0, false
		}
		return uint32(value), true
	case int64:
		if value < 0 || value > math.MaxUint32 {
			return 0, false
		}
		return uint32(value), true
	case float32:
		floatValue := float64(value)
		if floatValue < 0 || floatValue > math.MaxUint32 || floatValue != math.Trunc(floatValue) {
			return 0, false
		}
		return uint32(floatValue), true
	case float64:
		if value < 0 || value > math.MaxUint32 || value != math.Trunc(value) {
			return 0, false
		}
		return uint32(value), true
	case json.Number:
		if intValue, err := value.Int64(); err == nil {
			if intValue < 0 || intValue > math.MaxUint32 {
				return 0, false
			}
			return uint32(intValue), true
		}
		floatValue, err := value.Float64()
		if err != nil {
			return 0, false
		}
		if floatValue < 0 || floatValue > math.MaxUint32 || floatValue != math.Trunc(floatValue) {
			return 0, false
		}
		return uint32(floatValue), true
	case string:
		return coerceUInt32FromString(value)
	case []byte:
		return coerceUInt32FromBytes(value)
	default:
		return 0, false
	}
}

// CoerceBool attempts to convert common JSON-like values into bool.
func CoerceBool(value any) (bool, bool) {
	switch value := value.(type) {
	case bool:
		return value, true
	case uint:
		if value == 0 {
			return false, true
		}
		if value == 1 {
			return true, true
		}
		return false, false
	case uint64:
		if value == 0 {
			return false, true
		}
		if value == 1 {
			return true, true
		}
		return false, false
	case uint32:
		if value == 0 {
			return false, true
		}
		if value == 1 {
			return true, true
		}
		return false, false
	case int:
		if value == 0 {
			return false, true
		}
		if value == 1 {
			return true, true
		}
		return false, false
	case int64:
		if value == 0 {
			return false, true
		}
		if value == 1 {
			return true, true
		}
		return false, false
	case float64:
		if value == 0 {
			return false, true
		}
		if value == 1 {
			return true, true
		}
		return false, false
	case json.Number:
		if intValue, err := value.Int64(); err == nil {
			if intValue == 0 {
				return false, true
			}
			if intValue == 1 {
				return true, true
			}
			return false, false
		}
		return false, false
	case string:
		return coerceBoolFromString(value)
	case []byte:
		return coerceBoolFromBytes(value)
	default:
		return false, false
	}
}

func coerceBoolFromString(value string) (bool, bool) {
	trimmed := strings.TrimSpace(value)
	if parsed, err := strconv.ParseBool(trimmed); err == nil {
		return parsed, true
	}

	if parsed, ok := coerceUInt32FromString(trimmed); ok {
		if parsed == 0 {
			return false, true
		}
		if parsed == 1 {
			return true, true
		}
	}
	return false, false
}

func coerceBoolFromBytes(value []byte) (bool, bool) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return false, false
	}

	if parsed, ok := coerceBoolFromString(string(trimmed)); ok {
		return parsed, true
	}

	var nested any
	if err := json.Unmarshal(trimmed, &nested); err != nil {
		return false, false
	}
	return CoerceBool(nested)
}

func coerceUInt32FromString(value string) (uint32, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}

	parsedInt, err := strconv.ParseUint(trimmed, 10, 32)
	if err == nil {
		return uint32(parsedInt), true
	}

	parsedFloat, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}
	if parsedFloat < 0 || parsedFloat > math.MaxUint32 || parsedFloat != math.Trunc(parsedFloat) {
		return 0, false
	}
	return uint32(parsedFloat), true
}

func coerceUInt32FromBytes(value []byte) (uint32, bool) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 {
		return 0, false
	}

	if parsed, ok := coerceUInt32FromString(string(trimmed)); ok {
		return parsed, true
	}

	var nested any
	if err := json.Unmarshal(trimmed, &nested); err != nil {
		return 0, false
	}
	return CoerceUInt32(nested)
}
