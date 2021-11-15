package internal

import (
	"context"
	"io"
	"os"
	"strings"
	"time"
)

type File interface {
	io.ReadSeekCloser
	Stat() (os.FileInfo, error)
}

func OpenFile(filename string) (File, error) { return os.Open(filename) }

func OpenFileLoop(ctx context.Context, filename string, interval time.Duration) (File, error) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-t.C:
			if f, err := OpenFile(filename); err == nil {
				return f, nil
			}
		}
	}
}

func DropCRLF(buf string) string { return strings.TrimRight(buf, "\r\n") }
