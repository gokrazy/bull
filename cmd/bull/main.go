package main

import (
	"log"

	"github.com/gokrazy/bull/internal/bull"
)

func main() {
	if err := (&bull.Customization{}).Runbull(); err != nil {
		log.Fatal(err)
	}
}
