package config

import "testing"

func TestFindBackupSchedule(t *testing.T) {
	reg := &SourceRegistry{
		Sources: map[string]ResolvedSource{
			"a": {
				SourceDefinition: SourceDefinition{
					ID: "a",
					Schedules: []BackupSchedule{
						{ID: "one", RelativePath: "x", Schedule: "0 1 * * *"},
					},
				},
			},
		},
	}
	sch, ok := reg.FindBackupSchedule("one")
	if !ok || sch.SourceID != "a" {
		t.Fatalf("find: %+v ok=%v", sch, ok)
	}
	if _, ok = reg.FindBackupSchedule("missing"); ok {
		t.Fatal("expected missing")
	}
	if n := len(reg.ListAllBackupSchedules()); n != 1 {
		t.Fatalf("list all: %d", n)
	}
}