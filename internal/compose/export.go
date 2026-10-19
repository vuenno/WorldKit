package compose

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"

	"world-kit/internal/atomicfile"
)

// Export renders the composition, encodes it in format and writes it to path atomically.
// It returns the written file's info, so callers can record exactly what they wrote.
func Export(ctx context.Context, comp Composition, format Format, path string) (fs.FileInfo, error) {
	img, err := Render(ctx, comp, Decode)
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, img, format); err != nil {
		return nil, fmt.Errorf("encode %q: %w", path, err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := atomicfile.Write(path, encoded.Bytes()); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("check exported file %q: %w", path, err)
	}
	return info, nil
}
