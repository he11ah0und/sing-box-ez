package main

import (
	"log"
	"os"

	"sing-box-ez/internal/app"
	"sing-box-ez/internal/framework/localengine"
)

func init() {
	if err := localengine.LoadFromFS(localesFS, "locales"); err != nil {
		log.Fatalf("failed to load locales: %v", err)
	}
}

func main() {
	app.Run(os.Args[1:], runGUI)
}
