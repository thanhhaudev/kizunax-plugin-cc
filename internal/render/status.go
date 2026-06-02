package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/thanhhaudev/kizunax-plugin-cc/internal/job"
)

func RenderStatusList(jobs []job.Job) string {
	if len(jobs) == 0 {
		return "No jobs in this workspace.\n"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Jobs (%d):\n\n", len(jobs)))
	sb.WriteString("| ID | Kind | Status | Age | Duration | Target | Actions |\n")
	sb.WriteString("|---|---|---|---|---|---|---|\n")
	now := time.Now().UTC()
	for _, j := range jobs {
		age := now.Sub(j.CreatedAt).Round(time.Second)
		sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s | %s | %s | %s |\n",
			j.ID, j.Kind, statusIcon(j.Status), age,
			formatDuration(j.DurationMs), j.Request.Target.Label(), actionsFor(j)))
	}
	return sb.String()
}

func formatDuration(ms int64) string {
	if ms <= 0 {
		return ""
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	secs := float64(ms) / 1000.0
	if secs < 60 {
		return fmt.Sprintf("%.1fs", secs)
	}
	return fmt.Sprintf("%dm %ds", int(secs/60), int(secs)%60)
}

func actionsFor(j job.Job) string {
	var acts []string
	switch j.Status {
	case job.StatusRunning:
		acts = append(acts, fmt.Sprintf("`/kizunax:status %s`", j.ID))
		acts = append(acts, fmt.Sprintf("`/kizunax:cancel %s`", j.ID))
	case job.StatusCompleted, job.StatusFailed:
		acts = append(acts, fmt.Sprintf("`/kizunax:result %s`", j.ID))
	}
	return strings.Join(acts, "<br>")
}

func RenderJobDetail(j job.Job) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Job %s\n\n", j.ID))
	sb.WriteString(fmt.Sprintf("- Kind: `%s`\n", j.Kind))
	sb.WriteString(fmt.Sprintf("- Status: %s\n", statusIcon(j.Status)))
	sb.WriteString(fmt.Sprintf("- Target: %s\n", j.Request.Target.Label()))
	if j.Request.Focus != "" {
		sb.WriteString(fmt.Sprintf("- Focus: %s\n", j.Request.Focus))
	}
	sb.WriteString(fmt.Sprintf("- Created: %s\n", j.CreatedAt.Format(time.RFC3339)))
	if j.CompletedAt != nil {
		dur := j.CompletedAt.Sub(j.StartedAt).Round(time.Second)
		sb.WriteString(fmt.Sprintf("- Completed: %s (%s elapsed)\n", j.CompletedAt.Format(time.RFC3339), dur))
	}
	if j.PID > 0 {
		sb.WriteString(fmt.Sprintf("- Worker PID: %d\n", j.PID))
	}
	if j.LogPath != "" {
		sb.WriteString(fmt.Sprintf("- Log: %s\n", j.LogPath))
	}
	if j.Tokens != nil {
		sb.WriteString(fmt.Sprintf("- Tokens: in=%d out=%d total=%d\n",
			j.Tokens.Input, j.Tokens.Output, j.Tokens.Total))
	}
	if j.Error != "" {
		sb.WriteString(fmt.Sprintf("\n**Error**: %s\n", j.Error))
	}
	if len(j.Warnings) > 0 {
		sb.WriteString("\n**Warnings**:\n")
		for _, w := range j.Warnings {
			sb.WriteString("- " + w + "\n")
		}
	}
	if j.Result != nil {
		sb.WriteString(fmt.Sprintf("\nVerdict: **%s** with %d findings.\n", j.Result.Verdict, len(j.Result.Findings)))
		sb.WriteString(fmt.Sprintf("\nRun `/kizunax:result %s` to see full review.\n", j.ID))
	}
	return sb.String()
}

func statusIcon(s job.Status) string {
	switch s {
	case job.StatusRunning:
		return "⏳ running"
	case job.StatusCompleted:
		return "✓ completed"
	case job.StatusFailed:
		return "✗ failed"
	case job.StatusCancelled:
		return "■ cancelled"
	}
	return string(s)
}
