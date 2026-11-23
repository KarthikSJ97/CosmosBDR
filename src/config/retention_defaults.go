package config

// Defaults for retention GC (env overrides; see .env.example).
const (
	DefaultRetentionGCWorkflowID         = "retention_gc_daily"
	DefaultRetentionGCCronSchedule     = "0 3 * * *"
	DefaultRetentionGCItemBatchSize    = 200
	DefaultRetentionGCOrphanBatchSize  = 500
	DefaultRetentionGCExecutionTimeout = 3 // hours
)

func (c *Config) EffectiveRetentionGCWorkflowID() string {
	if c == nil || c.RetentionGCWorkflowID == "" {
		return DefaultRetentionGCWorkflowID
	}
	return c.RetentionGCWorkflowID
}

func (c *Config) EffectiveRetentionGCCronSchedule() string {
	if c == nil || c.RetentionGCCronSchedule == "" {
		return DefaultRetentionGCCronSchedule
	}
	return c.RetentionGCCronSchedule
}

func (c *Config) EffectiveRetentionGCItemBatchSize() int {
	if c == nil || c.RetentionGCItemBatchSize <= 0 {
		return DefaultRetentionGCItemBatchSize
	}
	return c.RetentionGCItemBatchSize
}

func (c *Config) EffectiveRetentionGCOrphanBatchSize() int {
	if c == nil || c.RetentionGCOrphanBatchSize <= 0 {
		return DefaultRetentionGCOrphanBatchSize
	}
	return c.RetentionGCOrphanBatchSize
}

func (c *Config) EffectiveRetentionGCExecutionTimeoutHours() int {
	if c == nil || c.RetentionGCExecutionTimeoutHours <= 0 {
		return DefaultRetentionGCExecutionTimeout
	}
	return c.RetentionGCExecutionTimeoutHours
}