package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/berquerant/gotailf"
)

func usage() string {
	return `Usage of gotailf:
  gotailf FILE

Follow the additional appended data to FILE and write it into the stdout.
`
}

func main() {
	if len(os.Args) != 2 || os.Args[1] == "-h" || os.Args[1] == "--help" {
		fmt.Fprintf(os.Stderr, usage())
		os.Exit(2)
		return
	}
	filename := os.Args[1]
	s := gotailf.NewContinueTailer(filename,
		gotailf.WithFlushInterval(200*time.Millisecond),
		gotailf.WithTailFromOriginWhenGone(true),
		gotailf.WithTailFromOriginWhenTruncated(true),
	)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	for line := range s.Tail(ctx) {
		fmt.Println(line)
	}
	stop()
	if err := s.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
