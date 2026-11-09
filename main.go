package main

import (
	"callbell/internal/desktop"
	"log"
)

func main() {
	if err := desktop.Run(); err != nil {
		log.Fatal(err)
	}
}
