package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"activity/pkg/proc"
)

const (
	agentdChatURL     = "http://127.0.0.1:8620/v1/chat"
	analyseModel      = "agentic-high"
	analyseTimeout    = 8 * time.Minute
	analyseTimeoutSec = 480
	promptCap         = 40
	promptMinRows     = 12
	promptRssFloor    = 100 << 20
	promptCpuFloor    = 5.0
)

const analyseSystem = `You advise which macOS processes to quit to free memory.
You only see live processes and the mean of each sample taken so far (a running sum divided by a count, not a timeline).
You cannot kill anything; you only recommend.
Never recommend killing kernel_task, launchd, WindowServer, loginwindow, sysmond, or login/security agents.
Prefer user apps, browsers, dev servers, and language runtimes whose average resident memory is large for what they are doing.
Rank at most 8 suggestions. Each line: pid, name, why, and whether it is safe to quit.
If nothing is worth quitting, say so. Do not invent processes that are not in the list.`

type chatRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	System         string `json:"system"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type chatReply struct {
	Text    string `json:"text"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// askAgentd sends one agentd3 chat completion. It does not open a conversation.
func askAgentd(ctx context.Context, procs []proc.Info, monitorSamples uint64) (string, error) {
	prompt := buildAnalysePrompt(procs, monitorSamples)
	return postChat(ctx, agentdChatURL, analyseModel, prompt)
}

func chatBody(model, prompt string) ([]byte, error) {
	return json.Marshal(chatRequest{
		Model:          model,
		Prompt:         prompt,
		System:         analyseSystem,
		TimeoutSeconds: analyseTimeoutSec,
	})
}

func postChat(ctx context.Context, url, model, prompt string) (string, error) {
	body, err := chatBody(model, prompt)
	if err != nil {
		return "", fmt.Errorf("encoding chat request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: analyseTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("chat call failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return readChatReply(resp)
}

func readChatReply(resp *http.Response) (string, error) {
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("reading chat reply: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat status %d: %s", resp.StatusCode, chatFailure(raw))
	}
	var reply chatReply
	if err := json.Unmarshal(raw, &reply); err != nil || strings.TrimSpace(reply.Text) == "" {
		return "", fmt.Errorf("chat reply missing text: %s", trimReply(raw))
	}
	return strings.TrimSpace(reply.Text), nil
}

func chatFailure(raw []byte) string {
	var reply chatReply
	if json.Unmarshal(raw, &reply) == nil {
		if reply.Error != "" {
			return trimReply([]byte(reply.Error))
		}
		if reply.Message != "" {
			return trimReply([]byte(reply.Message))
		}
	}
	return trimReply(raw)
}

func trimReply(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if len(text) > 400 {
		return text[:400]
	}
	return text
}

func buildAnalysePrompt(procs []proc.Info, monitorSamples uint64) string {
	var b strings.Builder
	fmt.Fprintf(&b, "monitor_samples=%d\n", monitorSamples)
	b.WriteString("per-process samples can be lower if the process started later.\n")
	b.WriteString("avg_rss is the mean resident size. avg_cpu is the mean of sampled CPU percent.\n")
	b.WriteString("Which of these should I get rid of?\n")
	for _, p := range selectForAnalyse(procs) {
		fmt.Fprintf(&b, "pid=%d name=%q rss=%s avg_rss=%s cpu=%.1f%% avg_cpu=%.1f%% samples=%d\n",
			p.PID, p.Name, formatBytes(p.RSS), formatBytes(p.AverageRSS()),
			p.CPU, p.AverageCPU(), p.Samples)
	}
	return b.String()
}

func selectForAnalyse(procs []proc.Info) []proc.Info {
	ranked := append([]proc.Info(nil), procs...)
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].AverageRSS() == ranked[j].AverageRSS() {
			return ranked[i].RSS > ranked[j].RSS
		}
		return ranked[i].AverageRSS() > ranked[j].AverageRSS()
	})
	return fillPromptRows(ranked)
}

func fillPromptRows(ranked []proc.Info) []proc.Info {
	picked := make([]proc.Info, 0, promptCap)
	seen := make(map[int32]struct{}, promptCap)
	for _, p := range ranked {
		if len(picked) >= promptCap || !noteworthy(p) {
			continue
		}
		picked = append(picked, p)
		seen[p.PID] = struct{}{}
	}
	for _, p := range ranked {
		if len(picked) >= promptMinRows || len(picked) >= promptCap {
			break
		}
		if _, ok := seen[p.PID]; ok {
			continue
		}
		picked = append(picked, p)
	}
	return picked
}

func noteworthy(p proc.Info) bool {
	return p.AverageRSS() >= promptRssFloor || p.RSS >= promptRssFloor ||
		p.CPU >= promptCpuFloor || p.AverageCPU() >= promptCpuFloor
}
