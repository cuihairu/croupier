package common

import (
	"fmt"
	"github.com/spf13/viper"
)

// LoadWithIncludes reads base config and merges includes in order.
func LoadWithIncludes(base string, includes []string) (*viper.Viper, error) {
	v := viper.New()
	if base != "" {
		v.SetConfigFile(base)
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}
	for _, inc := range includes {
		iv := viper.New()
		iv.SetConfigFile(inc)
		if err := iv.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("include %s: %w", inc, err)
		}
		v.MergeConfigMap(iv.AllSettings())
	}
	return v, nil
}

// mergeMaps recursively merges b into a.
func mergeMaps(a, b map[string]any) map[string]any {
	for k, vb := range b {
		if ma, ok := a[k].(map[string]any); ok {
			if mb, ok2 := vb.(map[string]any); ok2 {
				a[k] = mergeMaps(ma, mb)
				continue
			}
		}
		a[k] = vb
	}
	return a
}

// ApplySectionAndProfile extracts a section (server/agent/edge) and overlays profiles.<name> if present.
func ApplySectionAndProfile(v *viper.Viper, section, profile string) (*viper.Viper, error) {
	if section != "" {
		sub := v.Sub(section)
		if sub == nil {
			return nil, fmt.Errorf("section %s not found", section)
		}
		v = sub
	}
	if profile != "" {
		prof := v.Sub("profiles")
		if prof == nil {
			return nil, fmt.Errorf("profiles not found in section")
		}
		p := prof.Sub(profile)
		if p == nil {
			return nil, fmt.Errorf("profile %s not found", profile)
		}
		merged := mergeMaps(v.AllSettings(), p.AllSettings())
		nv := viper.New()
		nv.MergeConfigMap(merged)
		v = nv
	}
	return v, nil
}
