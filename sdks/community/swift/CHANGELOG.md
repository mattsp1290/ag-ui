# Changelog

## Unreleased

- Added a byte-oriented, frame-bounded SSE parser and a cancellable GET/POST
  watch client with a finite byte queue, raw named SSE frames, response
  validation, and per-request URLSession ownership.
- Added synthetic Agentcraft watch fixtures and boundary tests.
- Added AGUICore and AGUIClient to the AG-UI monorepo from the upstream Swift
  implementation, with local dojo chat integration. See [DERIVATION.md](DERIVATION.md)
  for provenance.
