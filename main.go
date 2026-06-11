package main

import (
	"os"

	"sing-box-ez/internal/app"
)

func main() {
	app.Run(os.Args[1:], localesFS, runGUI)
}
