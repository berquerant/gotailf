package internal_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/berquerant/gotailf/internal"
	"github.com/berquerant/gotailf/test"
	"github.com/stretchr/testify/assert"
)

func TestWatcher(t *testing.T) {
	t.Parallel()
	dir := test.NewTmpDir(t)
	defer dir.Remove(t)

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		w := internal.NewWatcher(dir.Path("not-found"), 50*time.Millisecond)
		_, err := w.Watch(context.TODO())
		assert.NotNil(t, err)
	})

	t.Run("no changes", func(t *testing.T) {
		t.Parallel()
		f := test.NewTmpFile(t)
		f.Close(t)
		defer f.Remove(t)
		w := internal.NewWatcher(f.Name(), 50*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
		defer cancel()
		eventC, err := w.Watch(ctx)
		assert.Nil(t, err)
		var changed bool
		for range eventC {
			changed = true
		}
		assert.False(t, changed)
	})

	toEventTypes := func(events []internal.FileChangeEvent) []internal.FileChangeEventType {
		got := make([]internal.FileChangeEventType, len(events))
		for i, ev := range events {
			got[i] = ev.Type()
		}
		return got
	}

	t.Run("removed", func(t *testing.T) {
		t.Parallel()
		f := test.NewTmpFile(t)
		f.Close(t)
		w := internal.NewWatcher(f.Name(), 50*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
		defer cancel()
		eventC, err := w.Watch(ctx)
		time.AfterFunc(80*time.Millisecond, func() {
			f.Remove(t)
		})
		assert.Nil(t, err)
		got := []internal.FileChangeEvent{}
		for ev := range eventC {
			got = append(got, ev)
		}
		assert.Equal(
			t,
			[]internal.FileChangeEventType{internal.FileChangeEventGone},
			toEventTypes(got),
		)
	})

	t.Run("moved", func(t *testing.T) {
		t.Parallel()
		f := test.NewTmpFile(t)
		f.Close(t)
		w := internal.NewWatcher(f.Name(), 50*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
		defer cancel()
		eventC, err := w.Watch(ctx)
		dest := fmt.Sprintf("%s-moved", f.Name())
		defer os.Remove(dest)
		time.AfterFunc(80*time.Millisecond, func() {
			assert.Nil(t, os.Rename(f.Name(), dest))
		})
		assert.Nil(t, err)
		got := []internal.FileChangeEvent{}
		for ev := range eventC {
			got = append(got, ev)
		}
		assert.Equal(
			t,
			[]internal.FileChangeEventType{internal.FileChangeEventGone},
			toEventTypes(got),
		)
	})

	t.Run("truncated", func(t *testing.T) {
		t.Parallel()
		f := test.NewTmpFile(t)
		fmt.Fprint(f.File(), "truncated")
		f.Close(t)
		defer f.Remove(t)
		w := internal.NewWatcher(f.Name(), 50*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
		defer cancel()
		eventC, err := w.Watch(ctx)
		time.AfterFunc(80*time.Millisecond, func() {
			assert.Nil(t, os.Truncate(f.Name(), 0))
		})
		assert.Nil(t, err)
		got := []internal.FileChangeEvent{}
		for ev := range eventC {
			got = append(got, ev)
		}
		assert.Equal(
			t,
			[]internal.FileChangeEventType{internal.FileChangeEventTruncated},
			toEventTypes(got),
		)
	})

	t.Run("appended", func(t *testing.T) {
		t.Parallel()
		f := test.NewTmpFile(t)
		defer func() {
			f.Close(t)
			f.Remove(t)
		}()
		w := internal.NewWatcher(f.Name(), 50*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
		defer cancel()
		eventC, err := w.Watch(ctx)
		time.AfterFunc(80*time.Millisecond, func() {
			fmt.Fprint(f.File(), "appended")
		})
		assert.Nil(t, err)
		got := []internal.FileChangeEvent{}
		for ev := range eventC {
			got = append(got, ev)
		}
		assert.Equal(
			t,
			[]internal.FileChangeEventType{internal.FileChangeEventAppended},
			toEventTypes(got),
		)
	})
}
