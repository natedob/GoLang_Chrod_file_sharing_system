package main

import (
	"Dist_Lab3/Chord"
	"os"
	"time"
)

func main() {
	Chord.Start(os.Args[1:])

	time.Sleep(time.Second)
}
