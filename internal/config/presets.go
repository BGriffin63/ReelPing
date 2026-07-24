package config

// Preset is a named bundle of monitoring thresholds.
type Preset struct {
	Name                 string
	Label                string
	CheckIntervalSeconds int
	TimeoutSeconds       int
	FailureThreshold     int
	RecoveryThreshold    int
	Description          string
}

// ConfirmSeconds returns the approximate outage confirmation time.
func (p Preset) ConfirmSeconds() int { return p.CheckIntervalSeconds * p.FailureThreshold }

// Presets returns the built-in monitoring presets keyed by name.
func Presets() map[string]Preset {
	return map[string]Preset{
		"fast": {
			Name: "fast", Label: "Fast",
			CheckIntervalSeconds: 15, TimeoutSeconds: 4,
			FailureThreshold: 4, RecoveryThreshold: 2,
			Description: "Frequent checks; confirms an outage in about 60 seconds.",
		},
		"balanced": {
			Name: "balanced", Label: "Balanced",
			CheckIntervalSeconds: 20, TimeoutSeconds: 5,
			FailureThreshold: 3, RecoveryThreshold: 2,
			Description: "Recommended default; confirms an outage in about 60 seconds.",
		},
		"conservative": {
			Name: "conservative", Label: "Conservative",
			CheckIntervalSeconds: 30, TimeoutSeconds: 5,
			FailureThreshold: 3, RecoveryThreshold: 2,
			Description: "Gentler polling; confirms an outage in about 90 seconds.",
		},
	}
}

// PresetOrder is the display order for presets, plus "custom".
var PresetOrder = []string{"fast", "balanced", "conservative", "custom"}

// ApplyPreset returns a copy of m with the named preset's thresholds applied.
// Unknown or "custom" names leave the thresholds untouched.
func ApplyPreset(m MonitoringConfig, name string) MonitoringConfig {
	p, ok := Presets()[name]
	if !ok {
		m.Preset = "custom"
		return m
	}
	m.Preset = p.Name
	m.CheckIntervalSeconds = p.CheckIntervalSeconds
	m.TimeoutSeconds = p.TimeoutSeconds
	m.FailureThreshold = p.FailureThreshold
	m.RecoveryThreshold = p.RecoveryThreshold
	return m
}
