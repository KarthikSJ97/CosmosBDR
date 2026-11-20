package sourceread

import (
	"context"
	"time"
)

// FileStat is filesystem metadata for an object at the source.
type FileStat struct {
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// SourceReader reads a single file from a backup source (local or SFTP).
type SourceReader interface {
	Stat(ctx context.Context) (FileStat, error)
	ReadAt(ctx context.Context, offset int64, length int) ([]byte, error)
	ContentSHA256(ctx context.Context) (string, error)
	Close() error
}