package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"activity/pkg/proc"
)

func TestBuildAnalysePromptUsesAverages(t *testing.T) {
	procs := []proc.Info{
		{PID: 1, Name: "kernel_task", RSS: 2 << 30, RSSSum: 4 << 30, Samples: 2, CPU: 1, CPUSum: 2},
		{PID: 99, Name: "python: leak", RSS: 8 << 30, RSSSum: 16 << 30, Samples: 4, CPU: 40, CPUSum: 80},
		{PID: 3, Name: "tiny helper", RSS: 4096, RSSSum: 8192, Samples: 2, CPU: 0.1, CPUSum: 0.2},
	}

	prompt := buildAnalysePrompt(procs, 40)
	if !strings.Contains(prompt, "monitor_samples=40") {
		t.Fatalf("prompt missing monitor sample count:\n%s", prompt)
	}
	if !strings.Contains(prompt, "pid=99") || !strings.Contains(prompt, "python: leak") {
		t.Fatalf("prompt missing the large process:\n%s", prompt)
	}
	if !strings.Contains(prompt, "4.0 GB") {
		t.Fatalf("prompt missing average RSS (16GB/4 = 4GB):\n%s", prompt)
	}
	if !strings.Contains(prompt, `name="kernel_task"`) {
		t.Fatalf("prompt missing quoted name:\n%s", prompt)
	}
}

func TestBuildAnalysePromptOmitsTinyWhenListIsFull(t *testing.T) {
	procs := []proc.Info{
		{PID: 99, Name: "python: leak", RSS: 8 << 30, RSSSum: 16 << 30, Samples: 4},
		{PID: 3, Name: "tiny helper", RSS: 4096, RSSSum: 8192, Samples: 2},
	}
	for i := int32(1); i <= promptMinRows; i++ {
		procs = append(procs, proc.Info{
			PID: 200 + i, Name: "hog", RSS: 200 << 20, RSSSum: 200 << 20, Samples: 1,
		})
	}
	prompt := buildAnalysePrompt(procs, 4)
	if strings.Contains(prompt, "tiny helper") {
		t.Fatalf("tiny process included after the noteworthy list was full:\n%s", prompt)
	}
	if !strings.Contains(prompt, "python: leak") {
		t.Fatalf("large process missing:\n%s", prompt)
	}
}

func TestSelectForAnalyseFillsWhenNothingIsLarge(t *testing.T) {
	procs := make([]proc.Info, 0, 15)
	for i := int32(1); i <= 15; i++ {
		procs = append(procs, proc.Info{
			PID: i, Name: "small", RSS: uint64(i) << 10, RSSSum: uint64(i) << 10, Samples: 1,
		})
	}
	got := selectForAnalyse(procs)
	if len(got) != promptMinRows {
		t.Fatalf("len = %d, want %d", len(got), promptMinRows)
	}
	if got[0].PID != 15 {
		t.Fatalf("highest average RSS should sort first, got pid %d", got[0].PID)
	}
}

func TestChatRequestUsesAgenticHigh(t *testing.T) {
	body := mustChatBody(t, analyseModel, "which process")
	if !strings.Contains(body, `"model":"agentic-high"`) {
		t.Fatalf("body = %s", body)
	}
	if strings.Contains(body, "conversation") {
		t.Fatalf("chat body must not open a conversation: %s", body)
	}
}

func TestPostChatUnknownModel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, err := postChat(ctx, agentdChatURL, "not-a-real-model", "hi")
	if err == nil {
		t.Fatal("expected the daemon to refuse an unknown model")
	}
	if !strings.Contains(err.Error(), "404") && !strings.Contains(err.Error(), "status") {
		t.Fatalf("error = %v, want a chat status failure", err)
	}
}

func TestSplitLinesDropsCarriageReturn(t *testing.T) {
	got := splitLines("one\r\n\ntwo\n")
	if len(got) != 3 || got[0] != "one" || got[1] != "" || got[2] != "two" {
		t.Fatalf("splitLines = %#v", got)
	}
}

func mustChatBody(t *testing.T, model, prompt string) string {
	t.Helper()
	raw, err := chatBody(model, prompt)
	if err != nil {
		t.Fatalf("chatBody: %v", err)
	}
	return string(raw)
}
