package main

import (
	"log"
	"os"

	app "gitcode.com/mindspore/mscli/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}
