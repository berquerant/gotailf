package gotailf

import (
	"context"

	"github.com/berquerant/gotailf/internal"
)

type continueTailer struct {
	filename string
	config   *Config
	tailer   Tailer
	err      error
}

// NewContinueTailer returns a new continue Tailer.
// When the target file is removed, waits that the file with the same name is created.
// When the target file is truncated, continues tailing the file.
func NewContinueTailer(filename string, opts ...Option) Tailer {
	config := newDefaultConfig()
	for _, opt := range opts {
		opt(config)
	}
	return &continueTailer{
		filename: filename,
		config:   config,
	}
}

func (s *continueTailer) Filename() string { return s.filename }
func (s *continueTailer) Pos() int64       { return s.tailer.Pos() }
func (s *continueTailer) Err() error {
	if s.err != nil {
		return s.err
	}
	return s.tailer.Err()
}
func (s *continueTailer) setErr(err error) { s.err = err }

func (s *continueTailer) open(ctx context.Context) error {
	f, err := internal.OpenFileLoop(ctx, s.filename, s.config.FlushInterval)
	if err != nil {
		return err
	}
	tailer, err := newTailerFromFile(f, s.config, internal.NewWatcher(s.filename, s.config.FlushInterval))
	if err != nil {
		return err
	}
	s.tailer = tailer
	return nil
}

func (s *continueTailer) Tail(ctx context.Context) <-chan string {
	resultC := make(chan string, s.config.BufferSize)
	go func() {
		s.loop(ctx, resultC)
		close(resultC)
	}()
	return resultC
}

func (s *continueTailer) loop(ctx context.Context, resultC chan<- string) {
	toOffset := func(isOrigin bool) int64 {
		if isOrigin {
			return 0
		}
		return -1 // EOF
	}
	for {
		if err := s.open(ctx); err != nil {
			s.setErr(err)
			return
		}
		for line := range s.tailer.Tail(ctx) {
			resultC <- line
		}
		switch s.tailer.Err() {
		case ErrFileGone:
			s.config.Offset = toOffset(s.config.TailFromOriginWhenGone)
			continue
		case ErrFileTruncated:
			s.config.Offset = toOffset(s.config.TailFromOriginWhenTruncated)
			continue
		default:
			s.setErr(s.tailer.Err())
			return
		}
	}
}
