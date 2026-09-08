package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/audio"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/config"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/document"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/imagegen"
	"github.com/ag-ui-protocol/ag-ui/sdks/community/go/example/server/internal/vision"
	"github.com/gofiber/fiber/v3"
)

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
	app := newApp(context.Background(), config.Config{CORS: true}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)),
		withMediaProviders(
			func(_ context.Context, req imagegen.GenerateRequest) (*imagegen.GenerateResult, error) {
				return &imagegen.GenerateResult{B64JSON: "cG5n", Prompt: req.Prompt}, nil
			},
			func(_ context.Context, req vision.AnalyzeRequest) (*vision.AnalyzeResult, error) {
				return &vision.AnalyzeResult{Text: "vision ok"}, nil
			},
			func(_ context.Context, req audio.TranscribeRequest) (*audio.TranscribeResult, error) {
				return &audio.TranscribeResult{Text: "audio ok"}, nil
			},
			func(_ context.Context, req document.AnalyzeRequest) (*document.AnalyzeResult, error) {
				return &document.AnalyzeResult{Text: "document ok"}, nil
			},
		))
	cases := []struct{ route, body, want string }{
		{"/image-gen", `{"messages":[{"role":"user","content":"draw a bird"}]}`, `"url":"data:image/png;base64,cG5n"`},
		{"/vision", `{"messages":[{"role":"user","content":[{"type":"image","source":{"type":"data","value":"aW1n","mimeType":"image/png"}}]}]}`, "vision ok"},
		{"/audio", `{"messages":[{"role":"user","content":[{"type":"audio","source":{"type":"data","value":"YXVkaW8=","mimeType":"audio/wav"}}]}]}`, "audio ok"},
		{"/document", `{"messages":[{"role":"user","content":[{"type":"document","source":{"type":"data","value":"cGRm","mimeType":"application/pdf"}}]}]}`, "document ok"},
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
}

func TestAppMediaProviderErrorIsRunError(t *testing.T) {
	app := newApp(context.Background(), config.Config{}, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), withMediaProviders(
		func(context.Context, imagegen.GenerateRequest) (*imagegen.GenerateResult, error) {
			return nil, errors.New("provider unavailable")
		}, nil, nil, nil))
	resp, err := app.Test(newRequest(t, http.MethodPost, "/image-gen", `{"messages":[{"role":"user","content":"draw"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(out), `"type":"RUN_ERROR"`) || !strings.Contains(string(out), "provider unavailable") {
		t.Fatalf("body=%s", out)
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
