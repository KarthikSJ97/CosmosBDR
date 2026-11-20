package sourceread

import (
	"context"
	"fmt"
	"strings"

	"cosmosbdr/src/config"
)

// Open returns a reader for itemPath. Empty sourceID means local filesystem at itemPath.
func Open(ctx context.Context, reg *config.SourceRegistry, sourceID, itemPath string) (SourceReader, error) {
	if strings.TrimSpace(sourceID) == "" {
		return NewLocalFileReader(itemPath), nil
	}
	if reg == nil {
		return nil, fmt.Errorf("source registry is not configured")
	}
	src, ok := reg.GetSource(sourceID)
	if !ok {
		return nil, fmt.Errorf("unknown source_id %q", sourceID)
	}
	switch strings.ToLower(src.Type) {
	case "local":
		return NewLocalFileReader(itemPath), nil
	case "sftp":
		return NewSFTPFileReader(ctx, src, itemPath)
	default:
		return nil, fmt.Errorf("unsupported source type %q", src.Type)
	}
}