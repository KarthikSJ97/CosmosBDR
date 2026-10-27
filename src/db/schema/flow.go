package schema

type FlowType string

const (
	FlowBackup  FlowType = "BACKUP"
	FlowRestore FlowType = "RESTORE"
	FlowSync    FlowType = "SYNC"
	FlowArchive FlowType = "ARCHIVE"
)
