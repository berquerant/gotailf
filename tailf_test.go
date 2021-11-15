package gotailf_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/berquerant/gotailf"
	"github.com/berquerant/gotailf/test"
	"github.com/stretchr/testify/assert"
)

func ExampleTailer() {
	f, _ := os.CreateTemp("", "")
	defer func() {
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
	}()

	s, err := gotailf.NewTailer(
		f.Name(),
		gotailf.WithFlushInterval(200*time.Millisecond),
	)
	if err != nil {
		panic(err)
	}
	go func() {
		t := time.NewTicker(50 * time.Millisecond)
		lines := []string{
			"first\n",
			"sec", "ond\n",
			"t", "hi", "rd", "\n",
			"fourth",
		}
		for _, line := range lines {
			<-t.C
			fmt.Fprint(f, line)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	for line := range s.Tail(ctx) {
		fmt.Println(line)
	}
	fmt.Println(s.Pos())
	if err := s.Err(); err != nil {
		panic(err)
	}
	// Output:
	// first
	// second
	// third
	// 19
}

func TestTailer(t *testing.T) {
	t.Parallel()
	dir := test.NewTmpDir(t)
	defer dir.Remove(t)

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		_, err := gotailf.NewTailer(dir.Path("notfound"))
		assert.NotNil(t, err)
	})

	type appendPair struct {
		content  string
		interval time.Duration
	}
	for _, tc := range []*struct {
		title    string
		deadline time.Duration
		opts     []gotailf.Option
		content  string
		appends  []appendPair
		want     []string
	}{
		{
			title:    "empty",
			deadline: 200 * time.Millisecond,
			opts:     []gotailf.Option{gotailf.WithFlushInterval(50 * time.Millisecond)},
			want:     []string{},
		},
		{
			title:    "no contents to tail",
			deadline: 200 * time.Millisecond,
			opts:     []gotailf.Option{gotailf.WithFlushInterval(50 * time.Millisecond)},
			content:  "content\n",
			want:     []string{},
		},
		{
			title:    "tail from origin",
			deadline: 200 * time.Millisecond,
			opts: []gotailf.Option{
				gotailf.WithFlushInterval(50 * time.Millisecond),
				gotailf.WithOffset(0),
			},
			content: "content\n",
			want:    []string{"content"},
		},
		{
			title:    "appended to empty",
			deadline: 200 * time.Millisecond,
			opts:     []gotailf.Option{gotailf.WithFlushInterval(50 * time.Millisecond)},
			appends: []appendPair{
				{
					content:  "append\n",
					interval: 20 * time.Millisecond,
				},
			},
			want: []string{"append"},
		},
		{
			title:    "appended",
			deadline: 200 * time.Millisecond,
			opts: []gotailf.Option{
				gotailf.WithFlushInterval(50 * time.Millisecond),
				gotailf.WithOffset(0),
			},
			content: "content\n",
			appends: []appendPair{
				{
					content:  "append1\n",
					interval: 20 * time.Millisecond,
				},
				{
					content:  "app", // without newline
					interval: 50 * time.Millisecond,
				},
				{
					content:  "end2\n",
					interval: 20 * time.Millisecond,
				},
			},
			want: []string{"content", "append1", "append2"},
		},
	} {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			f := test.NewTmpFile(t)
			fmt.Fprint(f.File(), tc.content)
			defer func() {
				f.Close(t)
				f.Remove(t)
			}()
			ctx, cancel := context.WithTimeout(context.TODO(), tc.deadline)
			defer cancel()
			s, err := gotailf.NewTailer(f.Name(), tc.opts...)
			assert.Nil(t, err)
			go func() {
				for _, p := range tc.appends {
					timer := time.NewTimer(p.interval)
					select {
					case <-ctx.Done():
						timer.Stop()
						return
					case <-timer.C:
						fmt.Fprint(f.File(), p.content)
					}
				}
			}()
			got := []string{}
			for line := range s.Tail(ctx) {
				got = append(got, line)
			}
			assert.Nil(t, s.Err())
			assert.Equal(t, tc.want, got)
		})
	}

	t.Run("removed", func(t *testing.T) {
		t.Parallel()
		f := test.NewTmpFile(t)
		fmt.Fprint(f.File(), "removed\n")
		f.Close(t)
		s, err := gotailf.NewTailer(f.Name(),
			gotailf.WithFlushInterval(50*time.Millisecond),
			gotailf.WithOffset(0),
		)
		assert.Nil(t, err)
		time.AfterFunc(80*time.Millisecond, func() {
			f.Remove(t)
		})
		got := []string{}
		for line := range s.Tail(context.TODO()) {
			got = append(got, line)
		}
		assert.Equal(t, gotailf.ErrFileGone, s.Err())
		assert.Equal(t, []string{"removed"}, got)
	})

	t.Run("moved", func(t *testing.T) {
		t.Parallel()
		f := test.NewTmpFile(t)
		fmt.Fprint(f.File(), "moved\n")
		f.Close(t)
		s, err := gotailf.NewTailer(f.Name(),
			gotailf.WithFlushInterval(50*time.Millisecond),
			gotailf.WithOffset(0),
		)
		assert.Nil(t, err)
		dest := fmt.Sprintf("%s-moved", f.Name())
		defer os.Remove(dest)
		time.AfterFunc(80*time.Millisecond, func() {
			assert.Nil(t, os.Rename(f.Name(), dest))
		})
		got := []string{}
		for line := range s.Tail(context.TODO()) {
			got = append(got, line)
		}
		assert.Equal(t, gotailf.ErrFileGone, s.Err())
		assert.Equal(t, []string{"moved"}, got)
	})

	t.Run("retail", func(t *testing.T) {
		t.Parallel()
		f := test.NewTmpFile(t)
		defer func() {
			f.Close(t)
			f.Remove(t)
		}()
		var pos int64
		{
			s, err := gotailf.NewTailer(f.Name(),
				gotailf.WithFlushInterval(50*time.Millisecond),
			)
			assert.Nil(t, err)
			time.AfterFunc(80*time.Millisecond, func() {
				fmt.Fprint(f.File(), "before\n")
			})
			ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
			got := []string{}
			for line := range s.Tail(ctx) {
				got = append(got, line)
			}
			cancel()
			assert.Nil(t, s.Err())
			assert.Equal(t, []string{"before"}, got)
			pos = s.Pos()
		}
		{
			s, err := gotailf.NewTailer(f.Name(),
				gotailf.WithFlushInterval(50*time.Millisecond),
				gotailf.WithOffset(pos),
			)
			assert.Nil(t, err)
			time.AfterFunc(80*time.Millisecond, func() {
				fmt.Fprint(f.File(), "after\n")
			})
			ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
			got := []string{}
			for line := range s.Tail(ctx) {
				got = append(got, line)
			}
			cancel()
			assert.Nil(t, s.Err())
			assert.Equal(t, []string{"after"}, got)
		}
	})
}
