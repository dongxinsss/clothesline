// A local, non-destructive recovery tool. It never prints record contents.
package main

import (
	"clothesline/internal/shelf"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: recover-history source-data-dir destination-data-dir")
		os.Exit(1)
	}
	s, e := shelf.Open(os.Args[2])
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	n, e := s.MergeFrom(os.Args[1])
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Printf("Recovered %d records without changing the source. Destination has %d active, %d archived records.\n", n, len(s.Snapshot().Items), len(s.Snapshot().History))
}
