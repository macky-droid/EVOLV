package main

import (
	"fmt"
	"strings"
	"time"
)

// hyperlink renders an OSC 8 clickable URL when the terminal is interactive.
// Piped output stays plain so `evolv help > notes.txt` does not collect escape
// sequences.
func hyperlink(url, text string) string {
	if !colorOn {
		return text
	}
	return "\033]8;;" + url + "\033\\" + text + "\033]8;;\033\\"
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func formatBoot(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func formatETA(d time.Duration) string {
	if d < 0 || d > 24*time.Hour {
		return "--"
	}
	if d < time.Second {
		return "0s"
	}
	secs := int(d.Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	return fmt.Sprintf("%dm%02ds", secs/60, secs%60)
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// progressBar is a tiny download meter. It throttles to ~12fps so a fast
// connection does not spend more time painting than transferring.
type progressBar struct {
	label  string
	total  int64
	start  time.Time
	last   time.Time
	frame  int
	active bool
}

func newProgressBar(label string, total int64) *progressBar {
	p := &progressBar{label: label, total: total, start: time.Now(), active: colorOn}
	if !p.active {
		fmt.Printf("  %s %s\n", paint(dim, "…"), label)
	}
	return p
}

func (p *progressBar) update(done, total int64) {
	if !p.active {
		return
	}
	if total > 0 {
		p.total = total
	}
	now := time.Now()
	if now.Sub(p.last) < 80*time.Millisecond && (p.total == 0 || done < p.total) {
		return
	}
	p.last = now
	p.frame++
	p.draw(done)
}

func (p *progressBar) draw(done int64) {
	spin := spinnerFrames[p.frame%len(spinnerFrames)]
	elapsed := time.Since(p.start).Seconds()
	speed := ""
	eta := "--"
	if elapsed > 0.2 {
		bps := float64(done) / elapsed
		speed = formatBytes(int64(bps)) + "/s"
		if p.total > done && bps > 0 {
			eta = formatETA(time.Duration(float64(p.total-done)/bps) * time.Second)
		}
	}
	if p.total <= 0 {
		fmt.Printf("\r\033[K  %s %-10s %s  %s", paint(cyan, spin), p.label, formatBytes(done), paint(dim, speed))
		return
	}
	pct := int(float64(done) / float64(p.total) * 100)
	if pct > 100 {
		pct = 100
	}
	filled := pct * 24 / 100
	bar := paint(cyan, strings.Repeat("█", filled)) + paint(dim, strings.Repeat("░", 24-filled))
	fmt.Printf("\r\033[K  %s %-10s %s %3d%%  %s / %s  %s  eta %s",
		paint(cyan, spin), p.label, bar, pct, formatBytes(done), formatBytes(p.total), paint(dim, speed), paint(dim, eta))
}

func (p *progressBar) finish() {
	if p.active {
		fmt.Print("\r\033[K")
	}
}
