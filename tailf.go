package gotailf

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/berquerant/gotailf/internal"
)

// Config represents Tailf() configurations.
type Config struct {
	// FlushInterval is the interval between checks of the new data of the target file.
	// Default is 1 second.
	FlushInterval time.Duration
	// Offset is for the next tail on the target file to offset (from the origin of the target file).
	// If the value is negative or over the size of the target, the end of the file.
	// Default is -1.
	Offset int64
	// BufferSize represents size of the read line buffer.
	// Default is 1000.
	BufferSize uint
	// TailFromOriginWhenGone is a flag to control tail continuation.
	// If true, continue tailing from the origin of the file when the file gone.
	// Default is false.
	TailFromOriginWhenGone bool
	// TailFromOriginWhenTruncated is a flag to control tail continuation.
	// If true, continue tailing from the origin of the file when the file truncated.
	// Default is true.
	TailFromOriginWhenTruncated bool
}

func newDefaultConfig() *Config {
	return &Config{
		FlushInterval:               time.Second,
		Offset:                      -1,
		BufferSize:                  1000,
		TailFromOriginWhenTruncated: true,
	}
}

type Option func(*Config)

// WithFlushInterval sets Config.FlushInterval.
func WithFlushInterval(interval time.Duration) Option {
	return func(c *Config) {
		c.FlushInterval = interval
	}
}

// WithOffset sets Config.Offset.
func WithOffset(offset int64) Option {
	return func(c *Config) {
		c.Offset = offset
	}
}

// WithBufferSize sets Config.BufferSize.
func WithBufferSize(size uint) Option {
	return func(c *Config) {
		c.BufferSize = size
	}
}

// WithTailFromOriginWhenGone sets Config.TailFromOriginWhenGone.
func WithTailFromOriginWhenGone(b bool) Option {
	return func(c *Config) {
		c.TailFromOriginWhenGone = b
	}
}

// WithTailFromOriginWhenTruncated sets Config.TailFromOriginWhenTruncated.
func WithTailFromOriginWhenTruncated(b bool) Option {
	return func(c *Config) {
		c.TailFromOriginWhenTruncated = b
	}
}

// Tailer provides an interface for tailing file.
type Tailer interface {
	// Tail starts tailing the file.
	// Yields appended lines.
	Tail(ctx context.Context) <-chan string
	// Filename returns the name of the target file.
	Filename() string
	// Pos returns the read offset.
	Pos() int64
	// Err returns the error of yielding.
	// This should be called when Tail() ends.
	Err() error
}

type tailer struct {
	// target filename.
	filename string
	// target file.
	file internal.File
	// file status watcher.
	watcher internal.Watcher
	config  *Config
	// tailing offset.
	pos int64
	err error
}

// NewTailer returns a new Tailer.
// The target file must be exist.
// When the target file is moved, removed or truncated then canceled.
func NewTailer(filename string, opts ...Option) (Tailer, error) {
	config := newDefaultConfig()
	for _, opt := range opts {
		opt(config)
	}
	return newTailer(filename, config, internal.NewWatcher(filename, config.FlushInterval))
}

func newTailer(filename string, config *Config, watcher internal.Watcher) (Tailer, error) {
	f, err := internal.OpenFile(filename)
	if err != nil {
		return nil, err
	}
	return newTailerFromFile(f, config, watcher)
}

func newTailerFromFile(f internal.File, config *Config, watcher internal.Watcher) (Tailer, error) {
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	pos := config.Offset
	if pos < 0 || pos > stat.Size() {
		pos = stat.Size() // tailing from EOF
	}
	offset, err := f.Seek(pos, 0)
	if err != nil {
		return nil, err
	}
	return &tailer{
		filename: stat.Name(),
		watcher:  watcher,
		file:     f,
		config:   config,
		pos:      offset,
	}, nil
}

func (s *tailer) Filename() string {
	if s == nil {
		return ""
	}
	return s.filename
}
func (s *tailer) Pos() int64 {
	if s == nil {
		return 0
	}
	return s.pos
}
func (s *tailer) Err() error {
	if s == nil {
		return nil
	}
	return s.err
}
func (s *tailer) addPos(pos int)   { s.pos += int64(pos) }
func (s *tailer) setErr(err error) { s.err = err }

func (s *tailer) Tail(ctx context.Context) <-chan string {
	resultC := make(chan string, s.config.BufferSize)
	go func() {
		s.loop(ctx, resultC)
		s.file.Close()
		close(resultC)
	}()
	return resultC
}

var (
	// ErrFileGone means that the file is moved or removed.
	ErrFileGone = errors.New("file gone")
	// ErrFileTruncated means that the file is truncated.
	ErrFileTruncated = errors.New("file truncated")
)

func (s *tailer) loop(ctx context.Context, resultC chan<- string) {
	var (
		r    = bufio.NewReader(s.file)
		buf  strings.Builder
		read = func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					x, err := r.ReadString('\n')
					if err != nil && !errors.Is(err, io.EOF) {
						return err
					}
					// EOF, not end in LF, yield next time.
					if err != nil {
						buf.WriteString(x)
						return nil
					}
					if buf.Len() > 0 {
						r := buf.String() + x
						s.addPos(len(r))
						resultC <- internal.DropCRLF(r)
						buf.Reset()
						continue
					}
					s.addPos(len(x))
					resultC <- internal.DropCRLF(x)
				}
			}
		}
	)

	if err := read(); err != nil {
		s.setErr(err)
		return
	}
	// Yield when the target file status is changed.
	eventC, err := s.watcher.Watch(ctx)
	if err != nil {
		s.setErr(err)
		return
	}
	for ev := range eventC {
		switch ev.Type() {
		case internal.FileChangeEventGone:
			s.setErr(ErrFileGone)
			return
		case internal.FileChangeEventTruncated:
			s.setErr(ErrFileTruncated)
			return
		case internal.FileChangeEventAppended:
			if err := read(); err != nil {
				s.setErr(err)
				return
			}
		default:
			panic("unreachable")
		}
	}
}
