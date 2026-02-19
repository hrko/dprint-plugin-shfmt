package dprint

type unresolvedConfigEntry struct {
	id     FormatConfigID
	config RawFormatConfig
}

func cloneConfigMap(config ConfigKeyMap) ConfigKeyMap {
	if len(config) == 0 {
		return make(ConfigKeyMap)
	}

	newConfig := make(ConfigKeyMap, len(config))
	for key, value := range config {
		newConfig[key] = value
	}
	return newConfig
}

func (r *Runtime[T]) setUnresolvedConfig(id FormatConfigID, config RawFormatConfig) {
	for i := range r.unresolvedConfigs {
		if r.unresolvedConfigs[i].id == id {
			r.unresolvedConfigs[i].config = config
			return
		}
	}

	r.unresolvedConfigs = append(r.unresolvedConfigs, unresolvedConfigEntry{
		id:     id,
		config: config,
	})
}

func (r *Runtime[T]) getUnresolvedConfig(id FormatConfigID) (RawFormatConfig, bool) {
	for i := range r.unresolvedConfigs {
		if r.unresolvedConfigs[i].id == id {
			return r.unresolvedConfigs[i].config, true
		}
	}
	return RawFormatConfig{}, false
}

func (r *Runtime[T]) removeUnresolvedConfig(id FormatConfigID) {
	for i := range r.unresolvedConfigs {
		if r.unresolvedConfigs[i].id != id {
			continue
		}

		lastIndex := len(r.unresolvedConfigs) - 1
		r.unresolvedConfigs[i] = r.unresolvedConfigs[lastIndex]
		r.unresolvedConfigs = r.unresolvedConfigs[:lastIndex]
		return
	}
}
