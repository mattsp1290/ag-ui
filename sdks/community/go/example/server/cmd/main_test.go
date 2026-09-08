package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
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

// routeModel is a stateless, content-driven fixture for exercising the real
// route assembly. WithTools returns a copy so concurrent route tests cannot
// mutate shared model state.
type routeModel struct{ toolNames map[string]bool }

func (m routeModel) Generate(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return schema.ConcatMessages(routeChunks(m, messages))
}
func (m routeModel) Stream(ctx context.Context, messages []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	chunks := routeChunks(m, messages)
	reader, writer := schema.Pipe[*schema.Message](len(chunks) + 1)
	go func() {
		defer writer.Close()
		for _, chunk := range chunks {
			writer.Send(chunk, nil)
		}
	}()
	return reader, nil
}
func (m routeModel) WithTools(infos []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	names := make(map[string]bool, len(infos))
	for _, info := range infos {
		names[info.Name] = true
	}
	return routeModel{toolNames: names}, nil
}
func routeChunks(m routeModel, messages []*schema.Message) []*schema.Message {
	if m.toolNames["render_card"] {
		for _, message := range messages {
			if message.Role == schema.Tool {
				return []*schema.Message{{Role: schema.Assistant, Content: "Card rendered."}}
			}
		}
		return []*schema.Message{{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "render-card-1", Type: "function", Function: schema.FunctionCall{Name: "render_card", Arguments: `{"title":"Test card","facts":[{"label":"Source","value":"fixture"}]}`}}}}}
	}
	return []*schema.Message{{Role: schema.Assistant, Content: "Fixture response with one clear step."}}
}

func testApp() *fiber.App {
	return newApp(context.Background(), config.Config{
		Host:      "127.0.0.1",
		Port:      8080,
		Provider:  "openai",
		Model:     "test-model",
		Workspace: ".",
		CORS:      true,
		GenUIPace: time.Millisecond,
	}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func testDeps(t *testing.T) *agent.Deps {
	t.Helper()
	tools, err := agent.NewReadOnlyToolset(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fixture := routeModel{}
	return &agent.Deps{Model: fixture, BaseModel: fixture, Tools: tools, Store: runstore.New(), AutoApprove: true, MaxIterations: 4, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Provider: "openai"}
}

func testConfig() config.Config {
	return config.Config{Host: "127.0.0.1", Port: 8080, Provider: "openai", Model: "test-model", Workspace: ".", CORS: true, GenUIPace: time.Millisecond}
}

func TestAppHealthRouteNoCredentials(t *testing.T) {
	resp, err := testApp().Test(newRequest(t, http.MethodGet, "/", ""))
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET / status = %d, body = %s", resp.StatusCode, body)
	}
	for _, want := range []string{"ag-ui-go-server-example is running", "/agentic_chat", "/human_in_the_loop", "/agentic_generative_ui", "/tool_based_generative_ui", "/shared_state", "/predictive_state_updates", "/image-gen", "/vision", "/audio", "/document", "/reasoning"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("GET / body missing %q: %s", want, body)
		}
	}
}

func TestAppRouteRegistrationMalformedJSON(t *testing.T) {
	for _, route := range []string{
		"/agentic",
		"/agentic_chat",
		"/backend_tool_rendering",
		"/human_in_the_loop",
		"/agentic_generative_ui",
		"/tool_based_generative_ui",
		"/shared_state",
		"/predictive_state_updates",
		"/agentic_chat_multimodal",
		"/image-gen",
		"/vision",
		"/audio",
		"/document",
		"/reasoning",
	} {
		t.Run(route, func(t *testing.T) {
			resp, err := testApp().Test(newRequest(t, http.MethodPost, route, "{"))
			if err != nil {
				t.Fatalf("POST %s: %v", route, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusNotFound {
				t.Fatalf("POST %s returned 404", route)
			}
			if resp.StatusCode != http.StatusBadRequest {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("POST %s status = %d, want 400; body = %s", route, resp.StatusCode, body)
			}
		})
	}
}

func TestAppCORSAllowsApprovalHeader(t *testing.T) {
	req := newRequest(t, http.MethodOptions, "/human_in_the_loop", "")
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Accept, Cache-Control, X-AG-Approval")

	resp, err := testApp().Test(req)
	if err != nil {
		t.Fatalf("OPTIONS /human_in_the_loop: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("preflight status = %d, body = %s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); !strings.Contains(strings.ToLower(got), "x-ag-approval") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want X-AG-Approval", got)
	}
	for _, want := range []string{"content-type", "accept", "cache-control"} {
		if got := strings.ToLower(resp.Header.Get("Access-Control-Allow-Headers")); !strings.Contains(got, want) {
			t.Fatalf("Access-Control-Allow-Headers = %q, want %s", got, want)
		}
	}
}

func TestAppMediaRoutesUseInjectedProviders(t *testing.T) {
	var imageReq imagegen.GenerateRequest
	var visionReq vision.AnalyzeRequest
	var audioReq audio.TranscribeRequest
	var documentReq document.AnalyzeRequest
	app := newApp(context.Background(), config.Config{CORS: true}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)),
		withMediaProviders(
			func(_ context.Context, req imagegen.GenerateRequest) (*imagegen.GenerateResult, error) {
				imageReq = req
				return &imagegen.GenerateResult{B64JSON: "cG5n", Prompt: req.Prompt}, nil
			},
			func(_ context.Context, req vision.AnalyzeRequest) (*vision.AnalyzeResult, error) {
				visionReq = req
				return &vision.AnalyzeResult{Text: "vision ok"}, nil
			},
			func(_ context.Context, req audio.TranscribeRequest) (*audio.TranscribeResult, error) {
				audioReq = req
				return &audio.TranscribeResult{Text: "audio ok"}, nil
			},
			func(_ context.Context, req document.AnalyzeRequest) (*document.AnalyzeResult, error) {
				documentReq = req
				return &document.AnalyzeResult{Text: "document ok"}, nil
			},
		))
	cases := []struct{ route, body, want string }{
		{"/image-gen", `{"messages":[{"role":"user","content":"draw a bird"}]}`, `"url":"data:image/png;base64,cG5n"`},
		{"/vision", `{"messages":[{"role":"user","content":[{"type":"image","source":{"type":"data","value":"aW1n","mimeType":"image/png"}},{"type":"text","text":"inspect bird"}]}]}`, "vision ok"},
		{"/audio", `{"messages":[{"role":"user","content":[{"type":"audio","source":{"type":"data","value":"YXVkaW8=","mimeType":"audio/wav"}}]}]}`, "audio ok"},
		{"/document", `{"messages":[{"role":"user","content":[{"type":"document","source":{"type":"data","value":"cGRm","mimeType":"application/pdf"}},{"type":"text","text":"summarize bird"}]}]}`, "document ok"},
	}
	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			resp, err := app.Test(newRequest(t, http.MethodPost, tc.route, tc.body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			out, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusOK || !strings.Contains(string(out), tc.want) {
				t.Fatalf("status=%d body=%s want=%q", resp.StatusCode, out, tc.want)
			}
		})
	}
	if imageReq.Prompt != "draw a bird" {
		t.Errorf("image request = %#v", imageReq)
	}
	if visionReq.ImageBase64 != "aW1n" || visionReq.MimeType != "image/png" || visionReq.Prompt != "inspect bird" {
		t.Errorf("vision request = %#v", visionReq)
	}
	if audioReq.AudioBase64 != "YXVkaW8=" || audioReq.MimeType != "audio/wav" {
		t.Errorf("audio request = %#v", audioReq)
	}
	if documentReq.PDFBase64 != "cGRm" || documentReq.MimeType != "application/pdf" || documentReq.Prompt != "summarize bird" {
		t.Errorf("document request = %#v", documentReq)
	}
}

func TestAppMediaProviderErrorIsRunError(t *testing.T) {
	failure := errors.New("provider unavailable")
	app := newApp(context.Background(), config.Config{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), withMediaProviders(
		func(context.Context, imagegen.GenerateRequest) (*imagegen.GenerateResult, error) { return nil, failure },
		func(context.Context, vision.AnalyzeRequest) (*vision.AnalyzeResult, error) { return nil, failure },
		func(context.Context, audio.TranscribeRequest) (*audio.TranscribeResult, error) { return nil, failure },
		func(context.Context, document.AnalyzeRequest) (*document.AnalyzeResult, error) { return nil, failure }))
	for _, tc := range []struct{ route, body string }{
		{"/image-gen", `{"messages":[{"role":"user","content":"draw"}]}`},
		{"/vision", `{"messages":[{"role":"user","content":[{"type":"image","source":{"type":"data","value":"aW1n"}}]}]}`},
		{"/audio", `{"messages":[{"role":"user","content":[{"type":"audio","source":{"type":"data","value":"YQ=="}}]}]}`},
		{"/document", `{"messages":[{"role":"user","content":[{"type":"document","source":{"type":"data","value":"ZA=="}}]}]}`},
	} {
		t.Run(tc.route, func(t *testing.T) {
			out := postBody(t, app, tc.route, tc.body)
			assertSSEFailure(t, out, "provider unavailable")
		})
	}
}

func TestAppMediaMissingInput(t *testing.T) {
	var calls atomic.Int32
	app := newApp(context.Background(), config.Config{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), withMediaProviders(
		func(context.Context, imagegen.GenerateRequest) (*imagegen.GenerateResult, error) {
			calls.Add(1)
			return &imagegen.GenerateResult{}, nil
		},
		func(context.Context, vision.AnalyzeRequest) (*vision.AnalyzeResult, error) {
			calls.Add(1)
			return &vision.AnalyzeResult{}, nil
		},
		func(context.Context, audio.TranscribeRequest) (*audio.TranscribeResult, error) {
			calls.Add(1)
			return &audio.TranscribeResult{}, nil
		},
		func(context.Context, document.AnalyzeRequest) (*document.AnalyzeResult, error) {
			calls.Add(1)
			return &document.AnalyzeResult{}, nil
		}))
	for _, tc := range []struct{ route, body, want string }{
		{"/image-gen", `{"messages":[]}`, "no user prompt"},
		{"/vision", `{"messages":[{"role":"user","content":"text only"}]}`, "no image part"},
		{"/audio", `{"messages":[{"role":"user","content":"text only"}]}`, "no audio part"},
		{"/document", `{"messages":[{"role":"user","content":"text only"}]}`, "no document part"},
	} {
		t.Run(tc.route, func(t *testing.T) {
			expectedStatus := http.StatusOK
			if tc.route == "/image-gen" {
				expectedStatus = http.StatusBadRequest
			}
			out := postBody(t, app, tc.route, tc.body, expectedStatus)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("body=%s want=%q", out, tc.want)
			}
			if tc.route == "/image-gen" {
				if strings.Contains(out, `"type":`) {
					t.Fatalf("image missing prompt should be HTTP JSON, got SSE: %s", out)
				}
			} else {
				assertSSEFailure(t, out, tc.want)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("provider called for missing input: %d", calls.Load())
	}
}

func TestAllElevenDestinationRoutesAcceptValidRequests(t *testing.T) {
	app := newApp(context.Background(), testConfig(), testDeps(t), slog.New(slog.NewTextHandler(io.Discard, nil)), withMediaProviders(
		func(context.Context, imagegen.GenerateRequest) (*imagegen.GenerateResult, error) {
			return &imagegen.GenerateResult{B64JSON: "cG5n"}, nil
		},
		func(context.Context, vision.AnalyzeRequest) (*vision.AnalyzeResult, error) {
			return &vision.AnalyzeResult{Text: "vision"}, nil
		},
		func(context.Context, audio.TranscribeRequest) (*audio.TranscribeResult, error) {
			return &audio.TranscribeResult{Text: "audio"}, nil
		},
		func(context.Context, document.AnalyzeRequest) (*document.AnalyzeResult, error) {
			return &document.AnalyzeResult{Text: "document"}, nil
		}))
	base := `{"threadId":"t","runId":"r","messages":[{"id":"u1","role":"user","content":"hello"}],"tools":[],"context":[],"forwardedProps":{},"state":{}}`
	cases := []struct{ route, body, event string }{
		{"/agentic_chat", base, "MESSAGES_SNAPSHOT"}, {"/human_in_the_loop", base, "MESSAGES_SNAPSHOT"},
		{"/agentic_generative_ui", base, "STATE_DELTA"},
		{"/tool_based_generative_ui", `{"threadId":"t","runId":"r","messages":[{"id":"u1","role":"user","content":"card"}],"tools":[{"name":"render_card","description":"render","parameters":{"type":"object"}}]}`, "TOOL_CALL_START"},
		{"/shared_state", base, "STATE_SNAPSHOT"}, {"/predictive_state_updates", base, "STATE_DELTA"}, {"/reasoning", base, "REASONING_START"},
		{"/image-gen", `{"messages":[{"role":"user","content":"draw"}]}`, "CUSTOM"},
		{"/vision", `{"messages":[{"role":"user","content":[{"type":"image","source":{"type":"data","value":"aW1n"}}]}]}`, "TEXT_MESSAGE_START"},
		{"/audio", `{"messages":[{"role":"user","content":[{"type":"audio","source":{"type":"data","value":"YQ=="}}]}]}`, "TEXT_MESSAGE_START"},
		{"/document", `{"messages":[{"role":"user","content":[{"type":"document","source":{"type":"data","value":"ZA=="}}]}]}`, "TEXT_MESSAGE_START"},
	}
	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			out := postBody(t, app, tc.route, tc.body)
			frames := decodeSSE(t, out)
			terminals, found := 0, false
			for _, frame := range frames {
				typ, _ := frame["type"].(string)
				if typ == "RUN_FINISHED" || typ == "RUN_ERROR" {
					terminals++
				}
				if typ == tc.event {
					found = true
				}
				if typ == "RUN_ERROR" {
					t.Errorf("unexpected RUN_ERROR: %v", frame)
				}
			}
			if terminals != 1 || !found {
				t.Fatalf("terminal=%d found %s=%v\n%s", terminals, tc.event, found, out)
			}
		})
	}
}

func TestAppValidSSELifecycleNoCredentials(t *testing.T) {
	body := `{"threadId":"t","runId":"r","messages":[{"id":"u1","role":"user","content":"Plan dinner"}],"tools":[],"context":[],"forwardedProps":{},"state":{}}`
	resp, err := testApp().Test(newRequest(t, http.MethodPost, "/agentic_generative_ui", body))
	if err != nil {
		t.Fatalf("POST /agentic_generative_ui: %v", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, out)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	for _, want := range []string{`"type":"RUN_STARTED"`, `"type":"RUN_FINISHED"`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("SSE body missing %q: %s", want, out)
		}
	}
}

func newRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func postBody(t *testing.T, app *fiber.App, route, body string, expectedStatus ...int) string {
	t.Helper()
	resp, err := app.Test(newRequest(t, http.MethodPost, route, body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	wantStatus := http.StatusOK
	if len(expectedStatus) > 0 {
		wantStatus = expectedStatus[0]
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("POST %s status=%d want=%d body=%s", route, resp.StatusCode, wantStatus, out)
	}
	wantContentType := "text/event-stream"
	if wantStatus == http.StatusBadRequest {
		wantContentType = "application/json"
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, wantContentType) {
		t.Fatalf("POST %s Content-Type=%q want %q", route, got, wantContentType)
	}
	return string(out)
}

func decodeSSE(t *testing.T, raw string) []map[string]any {
	t.Helper()
	var frames []map[string]any
	for _, line := range strings.Split(raw, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var frame map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &frame); err != nil {
			t.Fatalf("invalid SSE JSON %q: %v", line, err)
		}
		frames = append(frames, frame)
	}
	if len(frames) == 0 {
		t.Fatalf("no SSE frames: %s", raw)
	}
	return frames
}

func assertSSEFailure(t *testing.T, raw, message string) {
	t.Helper()
	started, failed, finished := 0, 0, 0
	frames := decodeSSE(t, raw)
	for _, frame := range frames {
		switch frame["type"] {
		case "RUN_STARTED":
			started++
		case "RUN_ERROR":
			failed++
		case "RUN_FINISHED":
			finished++
		}
	}
	if started != 1 || failed != 1 || finished != 0 || frames[len(frames)-1]["type"] != "RUN_ERROR" || !strings.Contains(raw, message) {
		t.Fatalf("invalid failure lifecycle started=%d failed=%d finished=%d want message %q\n%s", started, failed, finished, message, raw)
	}
}
