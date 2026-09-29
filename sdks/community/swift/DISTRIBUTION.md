# Standalone SwiftPM distribution

The authoritative development source is `sdks/community/swift` on the
`feat/swift-agentcraft` branch of `mattsp1290/ag-ui`. The package is published to
`https://github.com/mattsp1290/ag-ui-swift.git` by splitting that directory into
the root of a standalone Git repository. The distribution is maintained by
Matt Spurlin. Its `LICENSE` retains Perfect Aduh's MIT attribution; see
`DERIVATION.md` for the upstream source revision.

To update the distribution from a reviewed, committed AG-UI revision:

1. Run `swift build` and `swift test` in `sdks/community/swift`.
2. From the AG-UI repository root, run
   `git subtree split --prefix=sdks/community/swift <reviewed-ref>` and record
   the resulting full split SHA.
3. Check out the standalone distribution, fast-forward it to that split SHA,
   then push its default branch. Do not rewrite already published SHAs.
4. Create a fresh consumer that depends on the public Git URL at that exact
   SHA, resolve it with SwiftPM, and run macOS tests and an iOS build.
5. Record the AG-UI source SHA, split SHA and consumer results in the delivery
   record. A monorepo SHA is not a SwiftPM pin for this repository.

The published branch history is a split of package-path commits, so the
standalone repository's root contains `Package.swift`. `Sources`, `Tests`,
`LICENSE`, `DERIVATION.md`, and this procedure travel together.
