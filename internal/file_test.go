package internal_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/berquerant/gotailf/internal"
	"github.com/berquerant/gotailf/test"
	"github.com/stretchr/testify/assert"
)

func TestOpenFileLoop(t *testing.T) {
	t.Parallel()
	dir := test.NewTmpDir(t)
	defer dir.Remove(t)

	t.Run("not found", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
		defer cancel()
		_, err := internal.OpenFileLoop(ctx, dir.Path("notfound"), 50*time.Millisecond)
		assert.Equal(t, context.DeadlineExceeded, err)
	})

	t.Run("found", func(t *testing.T) {
		const name = "found"
		_, err := os.Create(dir.Path(name))
		assert.Nil(t, err)
		ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
		defer cancel()
		f, err := internal.OpenFileLoop(ctx, dir.Path(name), 50*time.Millisecond)
		assert.Nil(t, err)
		assert.NotNil(t, f)
		f.Close()
	})

	t.Run("created", func(t *testing.T) {
		const name = "created"
		time.AfterFunc(80*time.Millisecond, func() {
			_, err := os.Create(dir.Path(name))
			assert.Nil(t, err)
		})
		ctx, cancel := context.WithTimeout(context.TODO(), 200*time.Millisecond)
		defer cancel()
		f, err := internal.OpenFileLoop(ctx, dir.Path(name), 50*time.Millisecond)
		assert.Nil(t, err)
		assert.NotNil(t, f)
		f.Close()
	})
}
