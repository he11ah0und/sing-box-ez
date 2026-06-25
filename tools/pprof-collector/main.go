package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", "192.168.122.154:6060", "pprof server address")
	out := flag.String("out", "pprof-dumps", "output directory")
	flag.Parse()

	base := filepath.Join(*out, time.Now().Format("20060102_150405"))
	if err := os.MkdirAll(base, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	baseURL := "http://" + *addr + "/debug/pprof"

	fmt.Printf("[pprof-collector] waiting for server at %s...\n", *addr)
	for {
		resp, err := client.Get(baseURL + "/")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Printf("[pprof-collector] server is up, writing dumps to %s\n", base)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	goroutineTick := time.NewTicker(5 * time.Second)
	defer goroutineTick.Stop()
	heapTick := time.NewTicker(30 * time.Second)
	defer heapTick.Stop()
	cpuTick := time.NewTicker(60 * time.Second)
	defer cpuTick.Stop()

	n := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Println("[pprof-collector] stopped")
			return
		case <-goroutineTick.C:
			n++
			if err := dumpCtx(ctx, client, baseURL+"/goroutine?debug=2", filepath.Join(base, fmt.Sprintf("goroutine_%04d.txt", n))); err != nil {
				fmt.Printf("[pprof-collector] goroutine dump failed: %v\n", err)
			} else {
				fmt.Printf("[pprof-collector] goroutine dump #%04d saved\n", n)
			}
		case <-heapTick.C:
			name := filepath.Join(base, fmt.Sprintf("heap_%s.pb.gz", time.Now().Format("150405")))
			if err := dumpCtx(ctx, client, baseURL+"/heap", name); err != nil {
				fmt.Printf("[pprof-collector] heap dump failed: %v\n", err)
			} else {
				fmt.Printf("[pprof-collector] heap dump saved: %s\n", name)
			}
		case <-cpuTick.C:
			name := filepath.Join(base, fmt.Sprintf("cpu_%s.pb.gz", time.Now().Format("150405")))
			// /profile blocks for the requested duration; give it extra timeout.
			cpuClient := &http.Client{Timeout: 20 * time.Second}
			if err := dumpCtx(ctx, cpuClient, baseURL+"/profile?seconds=5", name); err != nil {
				fmt.Printf("[pprof-collector] cpu profile failed: %v\n", err)
			} else {
				fmt.Printf("[pprof-collector] cpu profile saved: %s\n", name)
			}
		}
	}
}

func dumpCtx(ctx context.Context, client *http.Client, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
