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

func TestContinueTailer(t *testing.T) {
	t.Parallel()

	t.Run("gone", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []*struct {
			title      string
			fromOrigin bool
			doRemove   bool
			content    string
			want       []string
		}{
			{
				title:      "rename from origin",
				fromOrigin: true,
				content:    "nines\n",
				want:       []string{"nines"},
			},
			{
				title:      "remove from origin",
				fromOrigin: true,
				doRemove:   true,
				content:    "nines\n",
				want:       []string{"nines"},
			},
			{
				title:   "rename from EOF",
				content: "nines\n",
				want:    []string{},
			},
			{
				title:    "remove from EOF",
				doRemove: true,
				content:  "nines\n",
				want:     []string{},
			},
		} {
			tc := tc
			t.Run(tc.title, func(t *testing.T) {
				t.Parallel()

				var (
					f    = test.NewTmpFile(t)
					src  = f.Name()
					dest = fmt.Sprintf("%s-moved", src)
				)
				f.Close(t)
				defer os.Remove(src)
				s := gotailf.NewContinueTailer(src,
					gotailf.WithFlushInterval(50*time.Millisecond),
					gotailf.WithTailFromOriginWhenGone(tc.fromOrigin),
				)
				ctx, cancel := context.WithTimeout(context.TODO(), 500*time.Millisecond)
				defer cancel()
				time.AfterFunc(80*time.Millisecond, func() {
					if tc.doRemove {
						f.Remove(t)
						return
					}
					assert.Nil(t, os.Rename(src, dest))
				})
				time.AfterFunc(170*time.Millisecond, func() {
					if tc.doRemove {
						_, err := os.Create(src)
						assert.Nil(t, err)
					} else {
						assert.Nil(t, os.Rename(dest, src))
					}
					f, err := os.OpenFile(src, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
					assert.Nil(t, err)
					fmt.Fprint(f, tc.content)
					f.Close()
				})
				got := []string{}
				for line := range s.Tail(ctx) {
					got = append(got, line)
				}
				assert.Nil(t, s.Err())
				assert.Equal(t, tc.want, got)
			})
		}
	})

	t.Run("truncate", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []*struct {
			title      string
			fromOrigin bool
			content    string
			want       []string
		}{
			{
				title:      "from origin",
				fromOrigin: true,
				content:    "second\n",
				want:       []string{"second"},
			},
			{
				title:   "EOF",
				content: "second\n",
				want:    []string{},
			},
		} {
			tc := tc
			t.Run(tc.title, func(t *testing.T) {
				t.Parallel()
				f := test.NewTmpFile(t)
				fmt.Fprintln(f.File(), "test continue tailer truncate")
				f.Close(t)
				defer os.Remove(f.Name())
				s := gotailf.NewContinueTailer(f.Name(),
					gotailf.WithFlushInterval(50*time.Millisecond),
					gotailf.WithTailFromOriginWhenTruncated(tc.fromOrigin),
				)
				time.AfterFunc(110*time.Millisecond, func() {
					assert.Nil(t, os.Truncate(f.Name(), 0))
					f, err := os.OpenFile(f.Name(), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
					assert.Nil(t, err)
					fmt.Fprint(f, tc.content)
					f.Close()
				})
				ctx, cancel := context.WithTimeout(context.TODO(), 300*time.Millisecond)
				defer cancel()
				got := []string{}
				for line := range s.Tail(ctx) {
					got = append(got, line)
				}
				assert.Nil(t, s.Err())
				assert.Equal(t, tc.want, got)
			})
		}
	})

	t.Run("from not found", func(t *testing.T) {
		t.Parallel()
		var filename string
		{
			f := test.NewTmpFile(t)
			filename = f.Name()
			f.Close(t)
			f.Remove(t)
		}
		s := gotailf.NewContinueTailer(filename, gotailf.WithFlushInterval(50*time.Millisecond))
		ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
		defer func() {
			cancel()
			os.Remove(filename)
		}()
		// first: empty file created
		time.AfterFunc(80*time.Millisecond, func() {
			_, err := os.Create(filename)
			assert.Nil(t, err)
		})
		// second: appended
		time.AfterFunc(120*time.Millisecond, func() {
			f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
			assert.Nil(t, err)
			fmt.Fprintln(f, "born")
			f.Close()
		})
		var got []string
		for line := range s.Tail(ctx) {
			got = append(got, line)
		}
		assert.Nil(t, s.Err())
		assert.Equal(t, []string{"born"}, got)
	})
}
