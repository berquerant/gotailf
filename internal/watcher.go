package internal

import (
	"context"
	"os"
	"time"
)

type (
	// Watcher watches changes of the file status.
	Watcher interface {
		Watch(ctx context.Context) (<-chan FileChangeEvent, error)
	}

	watcher struct {
		filename string
		size     int64
		interval time.Duration
	}

	FileChangeEventType int

	// FileChangeEvent is an event of the file status change.
	FileChangeEvent interface {
		Type() FileChangeEventType
		// Time is the last modified time.
		// When FileChangeEventGone, this is watched time.
		Time() time.Time
	}

	fileChangeEvent struct {
		typ FileChangeEventType
		tim time.Time
	}
)

func (s *fileChangeEvent) Type() FileChangeEventType { return s.typ }
func (s *fileChangeEvent) Time() time.Time           { return s.tim }

const (
	FileChangeEventUnknown FileChangeEventType = iota
	FileChangeEventTruncated
	FileChangeEventAppended
	FileChangeEventGone
)

func NewWatcher(filename string, interval time.Duration) Watcher {
	return &watcher{
		filename: filename,
		interval: interval,
	}
}

func (s *watcher) Watch(ctx context.Context) (<-chan FileChangeEvent, error) {
	stat, err := os.Stat(s.filename)
	if err != nil {
		return nil, err
	}
	s.size = stat.Size()

	eventC := make(chan FileChangeEvent)
	go func() {
		s.watch(ctx, eventC)
		close(eventC)
	}()
	return eventC, nil
}

func (s *watcher) watch(ctx context.Context, eventC chan<- FileChangeEvent) {
	t := time.NewTicker(s.interval)
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			stat, err := os.Stat(s.filename)
			if err != nil {
				eventC <- &fileChangeEvent{
					typ: FileChangeEventGone,
					tim: time.Now(),
				}
				return
			}
			switch {
			case s.size > stat.Size():
				eventC <- &fileChangeEvent{
					typ: FileChangeEventTruncated,
					tim: stat.ModTime(),
				}
			case s.size < stat.Size():
				eventC <- &fileChangeEvent{
					typ: FileChangeEventAppended,
					tim: stat.ModTime(),
				}
			}
			s.size = stat.Size()
		}
	}
}
