package cli

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// setupStatus prints a human-readable summary of the current setup-web worker
// (if any) and the last completed setup-web run (if any). Exit code is always 0.
func setupStatus() error {
	printCurrentWorker()
	fmt.Println()
	printLastResult()
	return nil
}

func printCurrentWorker() {
	s, err := loadSetupWebState()
	if err != nil || s.PID <= 0 {
		fmt.Println("Current worker: none.")
		return
	}
	proc, err := os.FindProcess(s.PID)
	if err != nil {
		fmt.Println("Current worker: none (stale state file).")
		return
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		fmt.Println("Current worker: none (stale state file).")
		return
	}
	remaining := time.Until(s.IdleDeadline)
	if remaining < 0 {
		remaining = 0
	}
	if s.StartedAt.IsZero() || s.IdleDeadline.IsZero() {
		fmt.Printf("Current worker: PID %d (started time unknown — pre-v0.6.5 state file).\n", s.PID)
		return
	}
	fmt.Printf("Current worker: PID %d, started %s, idle-exits %s (in %s).\n",
		s.PID,
		s.StartedAt.Local().Format("15:04:05"),
		s.IdleDeadline.Local().Format("15:04:05"),
		formatRemaining(remaining),
	)
}

func printLastResult() {
	r, err := loadSetupWebResult()
	if err != nil {
		fmt.Println("Last completed: none (no setup attempted recently).")
		return
	}
	completed := r.CompletedAt.Local().Format("15:04:05")
	switch r.Outcome {
	case setupWebSuccess:
		if r.ConfigPath != "" {
			fmt.Printf("Last completed: success — saved %s to %s.\n", completed, r.ConfigPath)
		} else {
			fmt.Printf("Last completed: success — saved %s.\n", completed)
		}
	case setupWebTimeout:
		fmt.Printf("Last completed: timeout — 5 min idle at %s. Re-run /kizunax:setup.\n", completed)
	case setupWebCancelled:
		fmt.Printf("Last completed: cancelled — signal at %s.\n", completed)
	default:
		fmt.Printf("Last completed: %s at %s — %s.\n", r.Outcome, completed, r.Message)
	}
}

// formatRemaining renders a duration as "Mm Ss" or "Ss" (drops the minute part if zero).
func formatRemaining(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	d = d.Round(time.Second)
	m := int(d / time.Minute)
	s := int((d % time.Minute) / time.Second)
	if m == 0 {
		return fmt.Sprintf("%ds", s)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}
