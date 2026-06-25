package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"sing-box-ez/internal/app"
)

func main() {
	// Optional localhost pprof server for diagnosing hangs.
	// Enable on Windows with: $env:SINGBOXEZ_PPROF=":6060"; .\sing-box-ez.exe
	if addr := os.Getenv("SINGBOXEZ_PPROF"); addr != "" {
		if addr == "1" {
			addr = "127.0.0.1:6060"
		}
		go func() {
			log.Printf("pprof listening on http://%s/debug/pprof/", addr)
			if err := http.ListenAndServe(addr, nil); err != nil {
				log.Printf("pprof server error: %v", err)
			}
		}()
	}

	app, err := app.New(os.Args[1:], runGUI)
	if err != nil {
		log.Fatal(err)
	}
	app.Run()
}
