# AG-UI Flutter Dojo

An eleven-page Flutter example using the local [Dart SDK](../) and [Go example server](../../go/example/server). It demonstrates streaming chat, client tools, local approval, generated cards, shared recipe state, predictive updates, media, and scripted reasoning.

## Install and run

Use Flutter **3.47.1** with its bundled Dart **3.13.1**, Git LFS, and Go 1.25 or newer. The app resolves `ag_ui` from `../`; it needs no sibling checkout, private library, Python, pnpm, or web-dojo runtime. The app floor is Dart 3.9 / Flutter 3.35; the parent SDK keeps its independent requirements.

```bash
# Repository root:
git lfs pull
cd sdks/community/dart/example
flutter pub get --enforce-lockfile
```

Terminal 1, with a valid `OPENAI_API_KEY` already set in that terminal:

```bash
# Repository root:
cd sdks/community/go/example/server
go run ./cmd
```

Terminal 2:

```bash
# Repository root:
cd sdks/community/dart/example
flutter run -d chrome
# Native macOS runner:
flutter run -d macos
```

The default URL is **http://127.0.0.1:8080**. Provider credentials belong only in the Go server environment. The regular executable requires a key at startup, including when you intend to use its scripted routes. The fixture below runs without credentials.

For Go `PORT=9090`, compile Flutter with the matching URL:

```bash
flutter run -d chrome --dart-define=AG_UI_BASE_URL=http://127.0.0.1:9090
```

Shell-exporting `AG_UI_BASE_URL` alone does not change the compiled app. Restart/rebuild after changing the define. Invalid HTTP(S) URLs produce a configuration message. Physical devices need a reachable server host and explicit Go `HOST`; Android emulators generally need their host-bridge address. The retained Android/iOS/Linux/Windows runners and mobile cleartext HTTP behavior are unverified.

## Destinations

| Page | POST route | Behavior |
| --- | --- | --- |
| Agentic Chat | `/agentic_chat` | Local `calculate` and `get_current_time` results continue on the same thread. |
| Human in the Loop | `/human_in_the_loop` | Approve or deny `request_approval`; the page toggle sends `?approval=off`. |
| Generative UI | `/agentic_generative_ui` | Scripted checklist progresses from pending through in-progress to completed. |
| Tool-based UI | `/tool_based_generative_ui` | `render_card` creates one title/facts card and a matching tool result. |
| Shared State | `/shared_state` | Edit the recipe between runs; the next request includes those edits. |
| Predictive Updates | `/predictive_state_updates` | A temporary draft appears before committed recipe steps and clears on termination. |
| Image Gen | `/image-gen` | Shows the `image_generated` data-URL result. |
| Vision | `/vision` | Inline image plus question. |
| Audio | `/audio` | Inline audio transcription. |
| Document Q&A | `/document` | Inline PDF plus question. |
| Reasoning Demo | `/reasoning` | Scripted balanced reasoning and answer events; this is demonstration text. |

Approval is a local demonstration: it sends the AG-UI tool result and continuation, but does not send an external message or execute the described action. Switching destinations disposes the page and starts a fresh thread. History and shared state are ephemeral. Only one exchange runs per page; incomplete streams and provider errors become visible errors, and client-tool continuations stop after eight follow-up runs. Attachments are limited to 5 MiB.

## Credential-free fixture

Install Docker Engine/CLI (Docker Desktop on macOS), resolve Flutter dependencies above, then:

```bash
# sdks/community/dart/example:
dart run test/helpers/go_server_container.dart
```

The Dart helper builds current Go source, starts the production route assembly with deterministic provider callbacks, and prints a mapped URL and shutdown deadline. Keep it running and pass that exact URL with `--dart-define=AG_UI_BASE_URL=...` when launching Flutter. Ctrl-C stops the server and removes its image, context, and Testcontainers reaper. Restart to obtain a new mapped port; a compiled app must be rebuilt with the new URL.

Use the exact prompt for the matching page:

| Page | Fixture prompt |
| --- | --- |
| Agentic Chat | `fixture:plain`, `fixture:calculate`, `fixture:time` |
| Human in the Loop | `fixture:approval` |
| Tool-based UI | `fixture:card` |
| Shared State | `fixture:shared-state` |
| Predictive Updates | `fixture:predictive`, `fixture:predictive-failure` |
| Checklist / Reasoning | Any message |
| Audio | Select an audio file; no text prompt is required. |
| Image / Vision / Document | `fixture:plain`, or `fixture:provider-failure` for an error |

Vision, audio, and document still require the corresponding file selection. The fixture validates production AG-UI input extraction but substitutes external analysis; it does not establish live-model accuracy. `fixture:delayed` and `fixture:interrupted` exercise lifecycle handling on Agentic Chat. Docker CLI and Testcontainers must use the same daemon: align `DOCKER_HOST`, `DOCKER_CONTEXT`, and `~/.testcontainers.properties` (`tc.host` takes precedence). Keep Ryuk enabled. No host credentials or workspace are mounted.

## Verify

```bash
# sdks/community/dart/example, using Flutter's bundled Dart:
flutter pub get --enforce-lockfile
flutter analyze
dart format --output=none --set-exit-if-changed lib test
flutter test --exclude-tags requires-go-server
flutter test test/go_server_contract_test.dart --tags requires-go-server --timeout 15m
flutter build web
flutter build macos --debug
```

The tagged suite requires Docker and fails with setup guidance if it is unavailable. Dart Testcontainers builds both Go images itself, drives the real Dart/Go SDK boundary, and verifies the regular executable's missing-key failure and sentinel-key scripted startup. Ordinary tests exclude the Docker tag. No live provider request is made by these gates.

The CI contract job also runs the tagged Docker failure tests in `test/docker_process_test.dart`. For development cleanup fault injection, run `dart --enable-asserts run test/helpers/verify_container_cleanup.dart` in a separate process. It verifies resource cleanup after an assertion failure and failed HTTP readiness. Standalone production-image instructions are in the [Go README](../../go/example/server#container).

The parent Dart CI job uses `dart pub get --no-example` and needs no Flutter. The Flutter app and Docker contract have separate jobs; the Go example has its own nested-module build/test/race/tidy job. Native commands are the standalone example interface; use Nx wrappers for repository automation when available.

## Troubleshooting

- Connection refused: confirm the Go host/port or fixture URL, then rebuild Flutter with the matching define.
- Browser request failure: confirm the Go CORS setting and that the page can reach the server. The default local server allows the Dart POST/SSE headers.
- Malformed or unsupported attachment: remove it and select a small supported file; image decoding failures show a fallback.
- Docker daemon mismatch: align the CLI context with the Testcontainers endpoint before rerunning. Never disable the contract tests silently.

## Source provenance

| Source | Revision | Disposition |
| --- | --- | --- |
| `github.com/mattsp1290/ag_ui_demo` | `1d1fa327d70d416cac909288497a6a07f94b313d` | Imported application, three tests, six platform runners and intentional assets. Included local optional Pods includes in both iOS xcconfigs, iOS/macOS Podfiles, and resolved the macOS Pod lock at the destination. |
| Prior monorepo Flutter migration | `9ef67a555c0f13ca2e88c4aecaf2c677fe6412fd` | Reused relative SDK wiring, unpublished package setup and CLI removal. Current source supersedes older page behavior; native runners are included. |
| `github.com/mattsp1290/ag-ui-go-server-example` | `ad756b7afd1c9abc4ebf422aaa28d338d154d2c3` | Reconciled against the existing monorepo server; retained destination route, iteration, file-read, identity and error-ordering fixes. See the Go README for the source-area dispositions. |

Both source checkouts remain intact. Source planning/IDE files, logs, generated SDK paths, caches, Pods and build products are excluded. The source sibling launcher and prior combined Go/Flutter launcher are intentionally retired. Native bundle/project identities remain unchanged.

## Verification record

Clean-checkout verification passed Flutter analysis, formatting, ordinary unit/widget tests, web and macOS debug builds, hydrated LFS assets, and the parent Dart tests without Flutter on its PATH. The committed real-Docker suite passed all 15 tests; two concurrent suite processes also passed with separate mapped ports and automatic resource cleanup. Deliberate wrong-route, wrong-tool-ID, and malformed-state-path mutations made their contract tests fail.

Interactive Chrome/macOS verification remains pending. The available browser runtime has no connected Chrome browser. The native app built and launched, but exposed no window to accessibility automation, so streaming navigation, file chooser operations and native request/result checks could not be observed. Builds do not establish browser transport, file-picker entitlements or end-to-end platform success.

Live-provider follow-up is **unperformed**, owner **Matt**: with a valid server-only key, exercise time/calculation, approval/denial, card generation, recipe edits, predictive steps, image generation, vision, audio transcription, and PDF questions. Record the model configuration and actual results without credential values. From the repository root, run existing opt-in provider tests with `cd sdks/community/go/example/server` then `GOWORK=off go test -tags integration ./...`; default Go tests do not run them.
