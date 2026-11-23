package config

import "testing"

func TestEffectiveRetentionGCDefaults(t *testing.T) {
	c := &Config{}
	if c.EffectiveRetentionGCCronSchedule() != DefaultRetentionGCCronSchedule {
		t.Fatalf("cron: %q", c.EffectiveRetentionGCCronSchedule())
	}
	if c.EffectiveRetentionGCItemBatchSize() != DefaultRetentionGCItemBatchSize {
		t.Fatalf("item batch: %d", c.EffectiveRetentionGCItemBatchSize())
	}
}

func TestEffectiveRetentionGCOverrides(t *testing.T) {
	c := &Config{
		RetentionGCCronSchedule:    "0 4 * * *",
		RetentionGCItemBatchSize:   50,
		RetentionGCOrphanBatchSize: 100,
	}
	if c.EffectiveRetentionGCCronSchedule() != "0 4 * * *" {
		t.Fatal("cron override")
	}
	if c.EffectiveRetentionGCItemBatchSize() != 50 {
		t.Fatal("item batch override")
	}
}