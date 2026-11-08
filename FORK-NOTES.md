# Fork mapo80/go-sdk

Fork of `github.com/modelcontextprotocol/go-sdk` for ARC (`arc-workspace`). The module path is unchanged; ARC uses it with `replace github.com/modelcontextprotocol/go-sdk => github.com/mapo80/go-sdk <tag>`.

- **Upstream base:** tag `v1.8.0`, commit `3f3b699b2b67e1ed033a63d6651671dab53c2d32`.
- **License:** unchanged (`LICENSE`, as upstream).
- **Branch of the changes:** `arc/json-v2`, merged into `main`.
- **Go:** `go 1.27` (`encoding/json/v2` and `encoding/json/jsontext` from the standard library).
- **Goal:** one JSON library in ARC's server build, the standard library's `encoding/json/v2`.

## Changes

| commit | change |
|---|---|
| `bb46196` | `internal/json` decodes with `encoding/json/v2` and the semantics of `encoding/json` (`DefaultOptionsV1`), names matched case-sensitively (`MatchCaseInsensitiveNames(false)`), as the `segmentio/encoding` decoder did with `DontMatchCaseInsensitiveStructFields`. `Unmarshal` reads the first value of the bytes (trailing data is not read, as before) with a pooled `jsontext.Decoder` over a `bytes.Buffer`, which jsontext parses without copying. `segmentio/encoding` and `segmentio/asm` are no longer in the build of the SDK; `segmentio/encoding` remains only as the reference of the equivalence test. |

## Equivalence

`TestDecoderMatchesSegmentio` decodes 38 inputs (JSON-RPC messages, case variants, duplicate names, exact and out-of-range numbers, nulls, invalid UTF-8, escapes and surrogates, trailing data, empty input, malformed input) into an interface, a map, a slice, a struct and `json.RawMessage`, through `Decoder` and through `Unmarshal`, against the segmentio decoder: equal values, equal error outcomes, `io.EOF` in the same cases. `TestDecoderReadsAStream` reads a stream of values in turn. Mutation check: with case-insensitive names the test fails. Upstream suite green (`go test ./...`), as on the base.

## Measures (8 November 2026, Apple M2 Pro, go1.27.1)

`BenchmarkUnmarshalWire` (two JSON-RPC messages, a `tools/call` request and its result, into the wire struct), benchstat, n=10:

| | segmentio (base) | json/v2 (fork) | |
|---|---|---|---|
| time/op | 8.55 µs | 2.62 µs | −69 % |
| bytes/op | 66 320 | 672 | −99 % |
| allocs/op | 13 | 8 | −38 % |
