package main

import (
	"log"

	"github.com/oxkrypton/PixelsMcp/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
