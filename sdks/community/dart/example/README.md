# AG-UI Flutter Dojo Demo

A Flutter web example for exercising AG-UI Dart SDK flows against a Dojo-compatible backend. The app uses the local SDK package with `ag_ui: path: ../` and demonstrates chat, client tool calls, human approval, live state projection, predictive updates, and multimodal request construction.

## Prerequisites

- Flutter SDK with a web target such as Chrome.
- An AG-UI backend exposing the routes listed below.
- CORS enabled for the Flutter web origin when running in a browser.

`AG_UI_BASE_URL` defaults to `http://127.0.0.1:8080`, matching the Go example server. Override it with `--dart-define` when the backend uses another URL.

## Setup

```bash
cd sdks/community/dart/example
flutter pub get
```

## Run

The quickest local path is to run the Go example server and Flutter app together:

```bash
cd sdks/community/dart/example
./scripts/run_with_go_server.sh
```

The script starts `sdks/community/go/example/server`, waits for `GET /`, and then runs Flutter with a matching `AG_UI_BASE_URL`.
It chooses the native current-platform Flutter device when that platform folder exists in the example; otherwise it uses the checked-in web target with `chrome`. Set `FLUTTER_DEVICE=...` to override the device, or `HOST`, `PORT`, and `AG_UI_BASE_URL` to override server binding and client URL.

The Go server's model-backed routes require the provider credentials expected by the server, such as `OPENAI_API_KEY`.

To run only Flutter against an already-running backend:

```bash
flutter run -d chrome --dart-define=AG_UI_BASE_URL=http://127.0.0.1:8080
```

The Flutter app does not start a backend. Start a compatible AG-UI server separately, then point `AG_UI_BASE_URL` at that server.

## Routes

| Page label | Endpoint | Required backend capability | Manual smoke prompt |
|------------|----------|-----------------------------|---------------------|
| Agentic Chat | `agentic_chat` | Text streaming and client tool calls | What is 12 * 7 + 3? |
| Human in the Loop | `human_in_the_loop` | Approval tool call and resume | Delete an old draft file |
| Generative UI | `agentic_generative_ui` | `STATE_SNAPSHOT` and `STATE_DELTA` | Make a plan for launching a docs site |
| Tool-based UI | `tool_based_generative_ui` | `render_card` client tool call | Show a card for Tokyo |
| Shared State | `shared_state` | Recipe state deltas | Add basil and make it serve 4 |
| Predictive Updates | `predictive_state_updates` | Predictive draft state and commit | Draft three recipe steps |
| Image Gen | `image-gen` | Text response or image generation route | Generate an image of a city skyline |
| Vision | `vision` | Multimodal image input | Attach a PNG and ask what is visible |
| Audio | `audio` | Multimodal audio input | Attach an audio file and ask for transcription |
| Document Q&A | `document` | Multimodal PDF input | Attach a PDF and ask for a summary |

## Verification

```bash
flutter pub get
dart format --set-exit-if-changed lib test
flutter analyze
flutter test
flutter build web
```

Do not commit generated `build/` output.

## CI Coverage

This example requires Flutter tooling. If repository CI remains Dart-only for the SDK package, treat this example as locally verified. To cover it in CI, add a Flutter job that runs the verification commands above from `sdks/community/dart/example`.

## Troubleshooting

- A `404` usually means the backend does not expose the selected endpoint.
- Browser CORS errors must be fixed in the backend CORS configuration.
- If tool-call pages do not continue after the first response, confirm the backend emits `MESSAGES_SNAPSHOT` with assistant `toolCalls` before `RUN_FINISHED`.
- For multimodal routes, choose files that match the extensions shown in the picker for the selected page.
