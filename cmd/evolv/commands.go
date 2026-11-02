package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dxn111/EVOLV/internal/update"
	"github.com/dxn111/EVOLV/internal/workspace"
)

func printHelp() {
	fmt.Println()
	fmt.Println("  " + paint(bold, "EVOLV") + "  " + paint(dim, "v"+workspace.Version+" · the workspace that rewrites itself"))
	fmt.Println()
	fmt.Println("  " + paint(dim, "usage"))
	rows := [][2]string{
		{"evolv", "boot the organism and open the canvas"},
		{"evolv update", "morph this binary to the latest release"},
		{"evolv update --check", "show current vs latest without downloading"},
		{"evolv download", "fetch the latest platform binary into this folder"},
		{"evolv status", "read a running organism's health card"},
		{"evolv export <dir>", "export the workspace as an .evolvpack"},
		{"evolv import <file>", "import an .evolvpack"},
		{"evolv version", "print the version"},
		{"evolv help", "show this screen"},
	}
	for _, row := range rows {
		fmt.Printf("  %s %-26s %s\n", paint(violet, "›"), paint(bold, row[0]), paint(dim, row[1]))
	}
	fmt.Println()
	fmt.Println("  " + paint(dim, "flags"))
	flags := [][2]string{
		{"--port N", "listen on a specific port (default 7373)"},
		{"--no-open", "do not launch a browser"},
		{"--open", "launch a browser even if config says no"},
		{"--reset", "restore the factory UI (genome + ledger survive)"},
		{"--version", "print the version and exit"},
	}
	for _, row := range flags {
		fmt.Printf("  %s %-26s %s\n", paint(dim, "·"), row[0], paint(dim, row[1]))
	}
	fmt.Println()
	fmt.Println("  " + paint(dim, "update flags: --check · --force · --json"))
	fmt.Println("  " + paint(dim, "download flags: --dir DIR · --force · --json"))
	fmt.Println("  " + paint(dim, "status flags: --port N · --json"))
	fmt.Println()
	fmt.Printf("  %s  %s\n\n", paint(dim, "github.com/dxn111/EVOLV"), paint(dim, "NO_COLOR=1 disables styling"))
}

func printUpdateHelp() {
	fmt.Println()
	fmt.Println("  " + paint(bold, "evolv update") + "  " + paint(dim, "morph the launcher in place"))
	fmt.Println("    --check   report current/latest without downloading")
	fmt.Println("    --force   reinstall the latest release even if already current")
	fmt.Println("    --json    machine-readable result")
	fmt.Println()
}

func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	check := fs.Bool("check", false, "check for a newer release without downloading")
	force := fs.Bool("force", false, "reinstall even when already current")
	jsonOut := fs.Bool("json", false, "machine-readable output")
	fs.SetOutput(os.Stdout)
	fs.Usage = printUpdateHelp
	if err := fs.Parse(args); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	info, err := update.Latest(ctx, workspace.Version)
	if err != nil {
		failWith(*jsonOut, "could not reach GitHub releases: %v", err)
	}
	latest := workspace.Version
	available := info != nil
	if available {
		latest = info.Version
	} else {
		rel, err := update.LatestRelease(ctx)
		if err != nil {
			failWith(*jsonOut, "could not resolve the latest release: %v", err)
		}
		latest = rel.Version
	}

	if *check {
		if *jsonOut {
			emitJSON(map[string]any{
				"current":   workspace.Version,
				"latest":    latest,
				"available": available,
				"url":       releaseURL(info, latest),
				"notes":     releaseNotes(info),
			})
			return
		}
		cardTop("update check")
		cardKV("current", "v"+workspace.Version, dim)
		cardKV("latest", "v"+latest, cyan)
		if available {
			cardKV("status", "update available", green)
		} else {
			cardKV("status", "already current", green)
		}
		cardBottom()
		fmt.Println()
		if available {
			fmt.Println("  " + paint(dim, "run "+paint(bold, "evolv update")+" to morph"))
			fmt.Println()
		}
		return
	}

	if !available && !*force {
		if *jsonOut {
			emitJSON(map[string]any{"ok": true, "updated": false, "version": workspace.Version})
			return
		}
		cardTop("update")
		cardKV("status", "already current", green)
		cardKV("version", "v"+workspace.Version, dim)
		cardBottom()
		fmt.Println()
		return
	}

	var bar *progressBar
	if !*jsonOut {
		size := int64(0)
		if info != nil {
			size = info.Size
		}
		bar = newProgressBar("download", size)
	}
	version, err := update.SelfUpdateWithProgress(ctx, func(done, total int64) {
		if bar != nil {
			bar.update(done, total)
		}
	})
	if bar != nil {
		bar.finish()
	}
	if err != nil {
		failWith(*jsonOut, "update failed: %v", err)
	}
	if *jsonOut {
		emitJSON(map[string]any{"ok": true, "updated": true, "previous": workspace.Version, "version": version})
		return
	}
	cardTop("update complete")
	cardKV("version", "v"+version, green)
	cardKV("genome", "preserved at ~/.evolv", dim)
	cardNote("→", "restart evolv to wake the new body", cyan)
	cardBottom()
	fmt.Println()
}

func runDownload(args []string) {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	dir := fs.String("dir", ".", "directory to write the binary into")
	force := fs.Bool("force", false, "replace an existing file")
	jsonOut := fs.Bool("json", false, "machine-readable output")
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Print("\n  evolv download [--dir DIR] [--force] [--json]\n\n")
	}
	if err := fs.Parse(args); err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var bar *progressBar
	if !*jsonOut {
		bar = newProgressBar("download", 0)
	}
	result, err := update.Download(ctx, *dir, *force, func(done, total int64) {
		if bar != nil {
			bar.update(done, total)
		}
	})
	if bar != nil {
		bar.finish()
	}
	if err != nil {
		failWith(*jsonOut, "download failed: %v", err)
	}
	if *jsonOut {
		emitJSON(result)
		return
	}
	cardTop("download complete")
	cardKV("version", "v"+result.Version, green)
	cardKV("file", result.Path, cyan)
	cardKV("size", formatBytes(result.Bytes), dim)
	cardNote("→", "chmod +x it and run it wherever you like", cyan)
	cardBottom()
	fmt.Println()
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	port := fs.Int("port", 0, "organism port (defaults to config.json)")
	jsonOut := fs.Bool("json", false, "print raw health JSON")
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Print("\n  evolv status [--port N] [--json]\n\n")
	}
	if err := fs.Parse(args); err != nil {
		return
	}
	if *port == 0 {
		*port = configuredPort()
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/health", *port)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		fail("bad status URL: %v", err)
	}
	resp, err := (&http.Client{Timeout: 2 * time.Second}).Do(req)
	if err != nil {
		if *jsonOut {
			emitJSON(map[string]any{"ok": false, "error": err.Error(), "port": *port})
		} else {
			fmt.Printf("\n  %s no organism is listening on 127.0.0.1:%d\n", paint(red, "✗"), *port)
			fmt.Println("    start it with: " + paint(bold, "evolv"))
			fmt.Println()
		}
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		fail("read status: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		fail("organism returned %s", resp.Status)
	}
	if *jsonOut {
		fmt.Println(string(body))
		return
	}
	var h map[string]any
	if err := json.Unmarshal(body, &h); err != nil {
		fail("bad health JSON: %v", err)
	}

	cardTop("organism status")
	cardKV("status", jsonString(h, "status"), green)
	cardKV("version", "v"+jsonString(h, "version"), dim)
	cardKV("provider", jsonString(h, "provider")+" · "+jsonString(h, "model"), violet)
	cardKV("uptime", formatETA(time.Duration(jsonInt(h, "uptime_seconds"))*time.Second), cyan)
	cardKV("requests", fmt.Sprintf("%d (%d errors)", jsonInt(h, "requests"), jsonInt(h, "errors")), dim)
	cardKV("sockets", fmt.Sprintf("%d live · %d subscribers", jsonInt(h, "live_connections"), jsonInt(h, "bus_subscribers")), dim)
	cardKV("evolutions", fmt.Sprintf("%d", jsonInt(h, "evolutions")), green)
	cardBottom()
	fmt.Println()
}

func configuredPort() int {
	home, err := os.UserHomeDir()
	if err != nil {
		return 7373
	}
	raw, err := os.ReadFile(filepath.Join(home, ".evolv", "config.json"))
	if err != nil {
		return 7373
	}
	var cfg struct {
		Port int `json:"port"`
	}
	if json.Unmarshal(raw, &cfg) == nil && cfg.Port > 0 {
		return cfg.Port
	}
	return 7373
}

func releaseURL(info *update.Info, fallback string) string {
	if info != nil && info.URL != "" {
		return info.URL
	}
	return "https://github.com/dxn111/EVOLV/releases/tag/v" + fallback
}

func releaseNotes(info *update.Info) string {
	if info == nil {
		return ""
	}
	return info.Notes
}

func jsonString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func jsonInt(m map[string]any, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

func emitJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func failWith(jsonOut bool, format string, args ...any) {
	if jsonOut {
		emitJSON(map[string]any{"ok": false, "error": fmt.Sprintf(format, args...)})
		os.Exit(1)
	}
	fail(format, args...)
}

func fail(format string, args ...any) {
	fmt.Printf("\n  %s %s\n\n", paint(red, "✗"), fmt.Sprintf(format, args...))
	os.Exit(1)
}
