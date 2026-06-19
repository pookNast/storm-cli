# storm-cli

A multi-perspective research CLI. Give it a topic and a role; it fans out to five
independent expert personas, maps where they agree and disagree, and returns a scored,
self-critiqued briefing — instead of the single "majority view" you get from one prompt.

Inspired by the [Stanford STORM](https://arxiv.org/abs/2402.14207) method (multi-perspective
question asking). Clean-room Go implementation — no Stanford code included.

## Why

A single LLM prompt collapses a topic to its modal answer. `storm-cli` keeps the
**disagreement** as a first-class output: the contradiction map (conflicts, consensus, and
blind spots) is the actual knowledge artifact, not the summary.

```
topic + role
   │
   ├─ Phase 1  Perspectives   5 personas in parallel (Practitioner, Academic,
   │                          Skeptic, Economist, Historian)
   ├─ Phase 2  Contradiction  conflicts · consensus · blind spots
   ├─ Phase 3  Synthesis      findings scored 1–10, hidden connection, role action,
   │                          frontier question
   └─ Phase 4  Self-critique  dominant bias · missing perspectives · weakest link
        │
        └─ briefing.json + briefing.md
```

## Install

```bash
go install github.com/pookNast/storm-cli/cmd/storm@latest
# or
git clone https://github.com/pookNast/storm-cli && cd storm-cli && make build
```

Requires Go 1.24+. Produces a single static binary, no CGO.

## Usage

```bash
storm research --topic "monorepo vs polyrepo for a small team" --role engineer
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--topic` | (required) | The research topic |
| `--role` | `analyst` | Whose lens the action items are written for (e.g. investor, clinician, policymaker) |
| `--format` | `both` | `json`, `md`, or `both` |
| `--out` | `./out` | Output directory (relative paths confined to CWD) |
| `--verbose` | `false` | Debug logging to stderr |

## Backend

`storm-cli` talks to any **OpenAI-compatible** `/chat/completions` endpoint — a local server
(Ollama, LM Studio, vLLM, llama.cpp) or a hosted API. Configure via environment:

| Env | Default | Notes |
|-----|---------|-------|
| `STORM_API_BASE` | `http://127.0.0.1:8400/v1` | Point this at your server (e.g. Ollama: `http://localhost:11434/v1`) |
| `STORM_API_KEY` | (empty) | Sent as a bearer token only when set; never logged or written to output |
| `STORM_MODEL` | `glm-4.5-air` | Model for personas / contradiction / critique |
| `STORM_SYNTH_MODEL` | `glm-5.1` | Model for synthesis (use your strongest) |
| `STORM_TIMEOUT_SEC` | `120` | Per-call timeout (1–3600) |

```bash
STORM_API_BASE=http://localhost:11434/v1 STORM_MODEL=llama3.1 \
  storm research --topic "..." --role analyst
```

## Output

`briefing.json` (structured) and `briefing.md` (human-readable). Both contain perspectives,
the contradiction map, scored findings, hidden connection, role-specific action, the frontier
question, and a self-critique.

> ⚠️ Evidence and citations in a briefing are LLM-generated and may be inaccurate or
> fabricated — verify independently before relying on them.

## Design notes

- **stdlib-first**: only external dependency is `golang.org/x/sync/errgroup` (parallel fan-out).
- **Resilient**: every phase retries once on malformed model output; a flaky self-critique
  degrades to an explicit "unavailable" note rather than discarding a good briefing.
- **Safe**: API keys never logged or written; `--out` paths are confined; response bodies capped.

## Development

```bash
make build   # build ./storm
make test    # go test ./...
make vet     # go vet
make fmt     # gofmt -w
```

## License

MIT — see [LICENSE](LICENSE).
