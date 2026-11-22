package config

// Defaults for Cadence fan-out (also used when env is missing or <= 0).
const (
	DefaultCadenceMaxConcurrentWorkflows       = 5
	DefaultCadenceWorkerMaxConcurrentActivities = 10
)

func (c *Config) EffectiveCadenceMaxConcurrentWorkflows() int {
	if c == nil || c.CadenceMaxConcurrentWorkflows <= 0 {
		return DefaultCadenceMaxConcurrentWorkflows
	}
	return c.CadenceMaxConcurrentWorkflows
}

// EffectiveCadenceWorkerMaxConcurrentActivities is the global activity execution cap for this worker process.
func (c *Config) EffectiveCadenceWorkerMaxConcurrentActivities() int {
	if c == nil || c.CadenceWorkerMaxConcurrentActivities <= 0 {
		return DefaultCadenceWorkerMaxConcurrentActivities
	}
	return c.CadenceWorkerMaxConcurrentActivities
}