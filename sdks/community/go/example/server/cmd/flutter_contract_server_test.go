//go:build contractserver

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/agent"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/audio"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/config"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/document"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/imagegen"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/runstore"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/vision"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/gofiber/fiber/v3"
)

// Fixture prompts are intentionally content-addressed: the model has no shared
// call counter, so concurrent tests and later turns remain deterministic.
const (
	promptPlain             = "fixture:plain"
	promptCalculate         = "fixture:calculate"
	promptTime              = "fixture:time"
	promptApproval          = "fixture:approval"
	promptCard              = "fixture:card"
	promptSharedState       = "fixture:shared-state"
	promptPredictive        = "fixture:predictive"
	promptPredictiveFailure = "fixture:predictive-failure"
	promptDelayed           = "fixture:delayed"
	promptInterrupted       = "fixture:interrupted"
	promptProviderFailure   = "fixture:provider-failure"
)

type contractModel struct {
	toolNames map[string]bool
}

func (m contractModel) Generate(ctx context.Context, messages []*schema.Message, options ...model.Option) (*schema.Message, error) {
	chunks, err := m.chunks(messages)
	if err != nil {
		return nil, err
	}
	return schema.ConcatMessages(chunks)
}

func (m contractModel) Stream(ctx context.Context, messages []*schema.Message, options ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if latestUserContains(messages, promptPredictiveFailure) || latestUserContains(messages, promptInterrupted) {
		reader, writer := schema.Pipe[*schema.Message](2)
		go func() {
			defer writer.Close()
			content := "Partial response before interruption."
			if latestUserContains(messages, promptPredictiveFailure) {
				content = "Uncommitted predictive draft."
			}
			writer.Send(&schema.Message{Role: schema.Assistant, Content: content}, nil)
			writer.Send(nil, errors.New("fixture interrupted stream failure"))
		}()
		return reader, nil
	}
	chunks, err := m.chunks(messages)
	if err != nil {
		return nil, err
	}
	reader, writer := schema.Pipe[*schema.Message](len(chunks) + 1)
	delayed := latestUserContains(messages, promptDelayed)
	go func() {
		defer writer.Close()
		for _, chunk := range chunks {
			if delayed {
				timer := time.NewTimer(350 * time.Millisecond)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					writer.Send(nil, ctx.Err())
					return
				}
			}
			if closed := writer.Send(chunk, nil); closed {
				return
			}
		}
	}()
	return reader, nil
}

func (m contractModel) WithTools(infos []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	names := make(map[string]bool, len(infos))
	for _, info := range infos {
		names[info.Name] = true
	}
	return contractModel{toolNames: names}, nil
}

func (m contractModel) chunks(messages []*schema.Message) ([]*schema.Message, error) {
	if latestUserContains(messages, promptPredictiveFailure) || latestUserContains(messages, promptInterrupted) {
		return nil, errors.New("fixture predictive provider failure")
	}
	if result := latestToolResult(messages); result != nil {
		switch result.ToolCallID {
		case fixtureToolID(messages, promptCalculate, "fixture-calculate-1"):
			return textChunks("The calculated result is 42."), nil
		case fixtureToolID(messages, promptTime, "fixture-time-1"):
			return textChunks("The fixture time is 09:30 UTC."), nil
		case fixtureToolID(messages, promptApproval, "fixture-approval-1"):
			if strings.Contains(strings.ToLower(result.Content), "false") || strings.Contains(strings.ToLower(result.Content), "deny") {
				return textChunks("The action was denied and was not performed."), nil
			}
			return textChunks("The action was approved and continued."), nil
		case fixtureToolID(messages, promptCard, "fixture-card-1"):
			return textChunks("The card was rendered once."), nil
		case fixtureToolID(messages, promptSharedState, "fixture-recipe-1"):
			return textChunks("The shared recipe was updated."), nil
		}
	}

	switch {
	case m.toolNames["apply_recipe_changes"] && latestUserContains(messages, promptSharedState):
		return toolChunks(fixtureToolID(messages, promptSharedState, "fixture-recipe-1"), "apply_recipe_changes", `{"title":"Fixture Soup","servings":4,"add_steps":["Serve warm."]}`), nil
	case m.toolNames["render_card"] && latestUserContains(messages, promptCard):
		return toolChunks(fixtureToolID(messages, promptCard, "fixture-card-1"), "render_card", `{"title":"Fixture card","facts":[{"label":"Mode","value":"stable"},{"label":"Executions","value":"one"}]}`), nil
	case m.toolNames["request_approval"] && latestUserContains(messages, promptApproval) && transcriptContains(messages, "careful assistant"):
		return toolChunks(fixtureToolID(messages, promptApproval, "fixture-approval-1"), "request_approval", `{"summary":"Send fixture message","action":"send"}`), nil
	case latestUserContains(messages, promptApproval):
		return textChunks("Approval is off; no approval tool was requested."), nil
	case m.toolNames["calculate"] && latestUserContains(messages, promptCalculate):
		return toolChunks(fixtureToolID(messages, promptCalculate, "fixture-calculate-1"), "calculate", `{"expression":"6*7"}`), nil
	case m.toolNames["get_current_time"] && latestUserContains(messages, promptTime):
		return toolChunks(fixtureToolID(messages, promptTime, "fixture-time-1"), "get_current_time", `{"timezone":"UTC"}`), nil
	case latestUserContains(messages, promptPredictive):
		return []*schema.Message{{Role: schema.Assistant, Content: "Prepare ingredients.\n"}, {Role: schema.Assistant, Content: "Serve warm."}}, nil
	case latestUserContains(messages, promptDelayed):
		return []*schema.Message{{Role: schema.Assistant, Content: "Delayed "}, {Role: schema.Assistant, Content: "fixture response."}}, nil
	case latestUserContains(messages, promptPlain):
		return textChunks("Plain fixture response."), nil
	default:
		return textChunks("Deterministic fixture response."), nil
	}
}

// Keep first-call IDs stable for readable assertions while later matching user
// turns receive a distinct identity derived solely from their transcript.
func fixtureToolID(messages []*schema.Message, prompt, base string) string {
	occurrences := 0
	for _, message := range messages {
		if message.Role == schema.User && strings.Contains(message.Content, prompt) {
			occurrences++
		}
	}
	if occurrences <= 1 {
		return base
	}
	return fmt.Sprintf("%s-turn-%d", base, occurrences)
}

func textChunks(content string) []*schema.Message {
	return []*schema.Message{{Role: schema.Assistant, Content: content}}
}

func toolChunks(id, name, arguments string) []*schema.Message {
	index := 0
	return []*schema.Message{{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{
		Index: &index,
		ID:    id,
		Type:  "function",
		Function: schema.FunctionCall{
			Name: name, Arguments: arguments,
		},
	}}}}
}

func transcriptContains(messages []*schema.Message, needle string) bool {
	for _, message := range messages {
		if strings.Contains(message.Content, needle) {
			return true
		}
	}
	return false
}

func latestUserContains(messages []*schema.Message, needle string) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == schema.User {
			return strings.Contains(messages[i].Content, needle)
		}
	}
	return false
}

func latestToolResult(messages []*schema.Message) *schema.Message {
	for i := len(messages) - 1; i >= 0; i-- {
		switch messages[i].Role {
		case schema.Tool:
			return messages[i]
		case schema.User, schema.Assistant:
			// A later visible turn supersedes historical tool results. This keeps a
			// follow-up user request content-addressed instead of replaying the prior
			// continuation merely because its tool result remains in history.
			return nil
		}
	}
	return nil
}

func TestContractModelIsContentAddressed(t *testing.T) {
	calculate := contractModel{toolNames: map[string]bool{"calculate": true}}
	chunks, err := calculate.chunks([]*schema.Message{schema.UserMessage(promptCalculate)})
	if err != nil || len(chunks) != 1 || len(chunks[0].ToolCalls) != 1 || chunks[0].ToolCalls[0].ID != "fixture-calculate-1" {
		t.Fatalf("calculate proposal = %#v, %v", chunks, err)
	}
	chunks, err = calculate.chunks([]*schema.Message{
		schema.UserMessage(promptCalculate),
		schema.ToolMessage(`{"result":42}`, "fixture-calculate-1"),
	})
	if err != nil || len(chunks) != 1 || chunks[0].Content != "The calculated result is 42." {
		t.Fatalf("calculate continuation = %#v, %v", chunks, err)
	}
	chunks, err = calculate.chunks([]*schema.Message{
		schema.UserMessage(promptCalculate),
		schema.ToolMessage(`{"result":42}`, "fixture-calculate-1"),
		{Role: schema.Assistant, Content: "The calculated result is 42."},
		schema.UserMessage(promptPlain),
	})
	if err != nil || len(chunks) != 1 || chunks[0].Content != "Plain fixture response." {
		t.Fatalf("later user turn replayed historical tool result: %#v, %v", chunks, err)
	}
	if _, err := (contractModel{}).chunks([]*schema.Message{schema.UserMessage(promptPredictiveFailure)}); err == nil {
		t.Fatal("predictive failure prompt did not fail")
	}
}

func TestContractModelRepeatsToolsWithDistinctIDs(t *testing.T) {
	for _, tc := range []struct{ prompt, name, id, answer string }{
		{promptCalculate, "calculate", "fixture-calculate-1", "The calculated result is 42."},
		{promptTime, "get_current_time", "fixture-time-1", "The fixture time is 09:30 UTC."},
		{promptApproval, "request_approval", "fixture-approval-1", "The action was approved and continued."},
		{promptCard, "render_card", "fixture-card-1", "The card was rendered once."},
		{promptSharedState, "apply_recipe_changes", "fixture-recipe-1", "The shared recipe was updated."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := contractModel{toolNames: map[string]bool{tc.name: true}}
			messages := []*schema.Message{schema.SystemMessage("careful assistant"), schema.UserMessage(tc.prompt)}
			first, err := fixture.chunks(messages)
			if err != nil || len(first) != 1 || len(first[0].ToolCalls) != 1 || first[0].ToolCalls[0].ID != tc.id {
				t.Fatalf("first proposal = %#v, %v", first, err)
			}
			messages = append(messages, first[0], schema.ToolMessage(`{"approved":true}`, tc.id), schema.AssistantMessage("Done", nil), schema.UserMessage(tc.prompt))
			second, err := fixture.chunks(messages)
			if err != nil || len(second) != 1 || len(second[0].ToolCalls) != 1 || second[0].ToolCalls[0].ID != tc.id+"-turn-2" {
				t.Fatalf("repeated proposal = %#v, %v", second, err)
			}
			messages = append(messages, second[0], schema.ToolMessage(`{"approved":true}`, tc.id+"-turn-2"))
			answer, err := fixture.chunks(messages)
			if err != nil || len(answer) != 1 || answer[0].Content != tc.answer || len(answer[0].ToolCalls) != 0 {
				t.Fatalf("repeat continuation = %#v, %v", answer, err)
			}
		})
	}
}

func TestServeFlutterContract(t *testing.T) {
	workspace := t.TempDir()
	tools, err := agent.NewReadOnlyToolset(workspace)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	fixture := contractModel{}
	deps := &agent.Deps{
		Model: fixture, BaseModel: fixture, Tools: tools, Store: runstore.New(),
		AutoApprove: false, MaxIterations: 8, Logger: logger,
		// This field enables multimodal conversion; both models above are local fixtures.
		Provider: "openai",
	}
	cfg := config.Config{
		Host: "0.0.0.0", Port: 8080, Provider: "fixture", Model: "content-addressed",
		Workspace: workspace, CORS: true, GenUIPace: 120 * time.Millisecond,
	}
	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	serverCtx, cancelDeadline := context.WithTimeout(signalCtx, 18*time.Minute)
	defer cancelDeadline()
	app := newApp(serverCtx, cfg, deps, logger, withMediaProviders(
		fixtureImageGenerator, fixtureVisionAnalyzer, fixtureAudioTranscriber, fixtureDocumentAnalyzer,
	))

	if err := app.Listen("0.0.0.0:8080", fiber.ListenConfig{
		DisableStartupMessage: true,
		GracefulContext:       serverCtx,
		ShutdownTimeout:       10 * time.Second,
	}); err != nil {
		t.Fatalf("contract server stopped unexpectedly: %v", err)
	}
}

func fixtureImageGenerator(_ context.Context, req imagegen.GenerateRequest) (*imagegen.GenerateResult, error) {
	if strings.Contains(req.Prompt, promptProviderFailure) {
		return nil, errors.New("fixture image provider failure")
	}
	return &imagegen.GenerateResult{
		B64JSON: "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=",
		Prompt:  req.Prompt,
	}, nil
}

func fixtureVisionAnalyzer(_ context.Context, req vision.AnalyzeRequest) (*vision.AnalyzeResult, error) {
	if strings.Contains(req.Prompt, promptProviderFailure) {
		return nil, errors.New("fixture vision provider failure")
	}
	return &vision.AnalyzeResult{Text: "Fixture image: a small blue square."}, nil
}

func fixtureAudioTranscriber(_ context.Context, req audio.TranscribeRequest) (*audio.TranscribeResult, error) {
	if strings.Contains(req.AudioBase64, "Zml4dHVyZS1wcm92aWRlci1mYWlsdXJl") {
		return nil, errors.New("fixture audio provider failure")
	}
	return &audio.TranscribeResult{Text: "Fixture audio transcription."}, nil
}

func fixtureDocumentAnalyzer(_ context.Context, req document.AnalyzeRequest) (*document.AnalyzeResult, error) {
	if strings.Contains(req.Prompt, promptProviderFailure) {
		return nil, errors.New("fixture document provider failure")
	}
	return &document.AnalyzeResult{Text: "Fixture document summary."}, nil
}
