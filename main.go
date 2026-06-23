package main

import (
	"log"
	"os"

	"sing-box-ez/internal/app"
)

func main() {
	app, err := app.New(os.Args[1:], runGUI)
	if err != nil {
		log.Fatal(err)
	}
	app.Run()
}
