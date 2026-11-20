package sourceread

import (
	"context"
	"fmt"
	"io"
)

// ReadFullSegment reads exactly up to length bytes at offset (tail chunks may be shorter).
func ReadFullSegment(ctx context.Context, r SourceReader, offset int64, length int) ([]byte, error) {
	buf := make([]byte, length)
	total := 0
	for total < length {
		n, err := r.ReadAt(ctx, offset+int64(total), length-total)
		if len(n) > 0 {
			copy(buf[total:], n)
			total += len(n)
		}
		if err != nil {
			if total > 0 && (err == io.EOF || err == io.ErrUnexpectedEOF) {
				break
			}
			if total == 0 {
				return nil, fmt.Errorf("read segment at %d: %w", offset, err)
			}
			break
		}
		if len(n) == 0 {
			break
		}
	}
	return buf[:total], nil
}