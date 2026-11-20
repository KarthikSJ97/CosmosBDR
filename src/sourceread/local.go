package sourceread

import (
	"context"
	"fmt"
	"os"

	"cosmosbdr/src/filehash"
)

// LocalFileReader reads from the container/local filesystem.
type LocalFileReader struct {
	path string
}

func NewLocalFileReader(path string) *LocalFileReader {
	return &LocalFileReader{path: path}
}

func (r *LocalFileReader) Stat(ctx context.Context) (FileStat, error) {
	_ = ctx
	info, err := os.Stat(r.path)
	if err != nil {
		return FileStat{}, err
	}
	return FileStat{
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

func (r *LocalFileReader) ReadAt(ctx context.Context, offset int64, length int) ([]byte, error) {
	_ = ctx
	f, err := os.Open(r.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	buf := make([]byte, length)
	n, err := f.ReadAt(buf, offset)
	if n > 0 {
		return buf[:n], nil
	}
	if err != nil {
		return nil, fmt.Errorf("local read at %d: %w", offset, err)
	}
	return buf[:0], nil
}

func (r *LocalFileReader) ContentSHA256(ctx context.Context) (string, error) {
	_ = ctx
	return filehash.ContentSHA256(r.path)
}

func (r *LocalFileReader) Close() error {
	return nil
}