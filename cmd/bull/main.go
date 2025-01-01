package main

import (
	"log"

	"github.com/gokrazy/bull/internal/bull"
)

func main() {
	if err := bull.Runbull(); err != nil {
		log.Fatal(err)
	}
}
