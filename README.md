# hlw

**h**arness **l**aunch **w**rapper — a small Go CLI that launches AI coding
agents (Claude Code, Codex, OpenCode, Pi) against configurable endpoints,
letting you pick the model at launch time.

```bash
hlw launch claude
```

![hlw picking a model and launching Claude Code](demo/hlw.gif)

One config file describes each agent: what to execute, which environment
variables it needs, where to fetch its model list, and how that agent expects to
be told which model to use. `hlw` fetches the list, shows a picker, and execs the
agent with everything wired up.

> **Status:** early but working. No tagged releases yet, so install from source.
> The config format may still change.

## Why

Running coding agents against a local or self-hosted inference server means
juggling a different set of environment variables and flags for every agent, and
re-editing them whenever you want a different model. `hlw` keeps that in one
place and makes the model a launch-time choice rather than a config edit.

It is a thin launcher: it sets environment variables, builds an argument list,
and hands over to the agent. It does not proxy requests or sit between the agent
and your endpoint.

## Requirements

- Go 1.27+ (to build)
- macOS or Linux
- The agent CLIs you want to launch, already installed and on your `PATH`
- An endpoint serving a model list — anything OpenAI-compatible
  (`/v1/models`), or Ollama (`/api/tags`)

## Installation

### From source

```bash
git clone https://github.com/first-it-consulting/hlw.git
cd hlw
make install
```

`make install` runs `go install`, which places the binary in `$(go env GOPATH)/bin`
— make sure that is on your `PATH`. No `sudo` required.

To build a binary in the working directory instead:

```bash
make build      # produces ./hlw
```

### From a release

Each release publishes binaries for macOS (arm64, amd64) and Linux (amd64,
arm64) with a `checksums.txt`:

```bash
tar -xzf hlw-<version>-darwin-arm64.tar.gz
install -m 755 hlw /usr/local/bin/hlw
```

### Homebrew

```bash
brew trust first-it-consulting/tap
brew install first-it-consulting/tap/hlw
```

The `brew trust` line is required. Since Homebrew 6.0.0, formulae from
non-official taps are refused until you trust them, and `brew tap` on its own
fails with `Refusing to load formula ... from untrusted tap` — repeated once per
platform, then `Cannot tap ...: invalid syntax in tap!` — and leaves the tap
uninstalled. Trust is per-machine and cannot be granted by the tap, so this is
needed on every machine, and it is worth understanding rather than pasting:
trusting a tap means its code may run with your user's privileges whenever
Homebrew loads it. See [Tap Trust](https://docs.brew.sh/Tap-Trust).

`brew install` taps automatically, so no separate `brew tap` is needed.

Homebrew resolves a tap name by prepending `homebrew-`, so `first-it-consulting/tap`
means the [`first-it-consulting/homebrew-tap`](https://github.com/first-it-consulting/homebrew-tap)
repository. That is where the formula Homebrew installs is served from — this
repo cannot double as its own tap, because the name would have to be
`homebrew-hlw`. The formula is generated from
[`Formula/hlw.rb.template`](Formula/hlw.rb.template) here and pushed to the tap
on release, so no version or checksum is ever committed to this repo and the
two cannot drift apart.

## Quick start

1. Create `~/.config/hlw/config.json` — start from
   [`config.example.json`](config.example.json):

   ```bash
   mkdir -p ~/.config/hlw
   cp config.example.json ~/.config/hlw/config.json
   chmod 600 ~/.config/hlw/config.json
   ```

   The file can hold API tokens, so keep it readable only by you. Better still,
   export the token in your shell and let `hlw` read it from the environment —
   see [Keeping secrets out of the config file](#keeping-secrets-out-of-the-config-file).

2. Point `endpoint` at your server and adjust the executables to match what
   you have installed.

3. Check it parses:

   ```bash
   hlw list
   ```

4. Launch:

   ```bash
   hlw launch claude
   ```

## Configuration

`hlw` reads `~/.config/hlw/config.json`. A JSON Schema for editor completion and
validation is in [`config.schema.json`](config.schema.json).

```json
{
  "harnesses": {
    "claude": {
      "executable": "claude",
      "description": "Claude Code",
      "args": [
        "--permission-mode",
        "bypassPermissions",
        "--disallowedTools",
        "LSP"
      ],
      "endpoint": "http://127.0.0.1:4000",
      "modelAuthToken": "{{env:ANTHROPIC_AUTH_TOKEN}}",
      "modelEnvVars": [
        "ANTHROPIC_DEFAULT_OPUS_MODEL",
        "ANTHROPIC_DEFAULT_SONNET_MODEL",
        "ANTHROPIC_DEFAULT_HAIKU_MODEL"
      ],
      "envVars": {
        "ANTHROPIC_BASE_URL": "{{endpoint}}",
        "API_TIMEOUT_MS": "3000000",
        "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
        "CLAUDE_CODE_MAX_CONTEXT_TOKENS": "200000",
        "CLAUDE_CODE_AUTO_COMPACT_WINDOW": "158000",
        "CLAUDE_AUTOCOMPACT_PCT_OVERRIDE": "80"
      }
    },
    "codex": {
      "executable": "codex",
      "description": "OpenAI Codex",
      "endpoint": "http://127.0.0.1:4000",
      "modelAuthToken": "{{env:OMLX_API_KEY}}",
      "modelArgs": [
        "-m",
        "{{model}}",
        "--config=model_context_window={{contextWindow}}"
      ]
    },
    "opencode": {
      "executable": "opencode",
      "description": "OpenCode",
      "endpoint": "http://127.0.0.1:4000",
      "modelAuthToken": "{{env:OMLX_API_KEY}}",
      "modelArgs": [
        "--model",
        "omlx/{{model}}"
      ]
    },
    "pi": {
      "executable": "pi",
      "description": "Pi",
      "endpoint": "http://127.0.0.1:4000",
      "modelAuthToken": "{{env:OMLX_API_KEY}}",
      "modelArgs": [
        "--model",
        "omlx/{{model}}"
      ]
    },
    "claude-ollama": {
      "executable": "claude",
      "description": "Claude Code via Ollama",
      "args": [
        "--permission-mode",
        "bypassPermissions"
      ],
      "endpoint": "http://127.0.0.1:11434",
      "modelEnvVars": [
        "ANTHROPIC_DEFAULT_OPUS_MODEL",
        "ANTHROPIC_DEFAULT_SONNET_MODEL",
        "ANTHROPIC_DEFAULT_HAIKU_MODEL"
      ],
      "envVars": {
        "ANTHROPIC_BASE_URL": "{{endpoint}}",
        "ANTHROPIC_AUTH_TOKEN": "ollama"
      }
    },
    "codex-ollama": {
      "executable": "codex",
      "description": "Codex via Ollama",
      "args": [
        "-c",
        "model_provider=ollama"
      ],
      "endpoint": "http://127.0.0.1:11434",
      "modelArgs": [
        "-m",
        "{{model}}"
      ]
    },
    "opencode-ollama": {
      "executable": "opencode",
      "description": "OpenCode via Ollama",
      "endpoint": "http://127.0.0.1:11434",
      "modelArgs": [
        "--model",
        "ollama/{{model}}"
      ]
    },
    "pi-ollama": {
      "executable": "pi",
      "description": "Pi via Ollama",
      "endpoint": "http://127.0.0.1:11434",
      "modelArgs": [
        "--model",
        "ollama/{{model}}"
      ]
    }
  }
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `executable` | string | Yes | Path or name of the executable to run |
| `args` | array | No | Default command-line arguments (default: `[]`) |
| `envVars` | object | No | Environment variables to inject into the subprocess; values may use `{{endpoint}}` and `{{env:NAME}}` |
| `endpoint` | string | No | Base URL of an OpenAI-compatible server; the model list is read from `<endpoint>/v1/models` |
| `modelURL` | string | No | Full model-list URL, for servers that serve it elsewhere. Overrides `endpoint` |
| `modelEnvVars` | array | No | Env vars set to the selected model id |
| `modelArgs` | array | No | Args appended after `args`, with placeholders substituted |
| `modelAuthToken` | string | No | Bearer token for the model-list request; supports `{{env:NAME}}`. Omitted means unauthenticated |
| `description` | string | No | Optional description of the harness |

### Model list formats

Point a harness at a server with `endpoint` — the base URL. The model list is
read from `<endpoint>/v1/models`, which every OpenAI-compatible server provides:

```json
"endpoint": "http://127.0.0.1:4000"     -> http://127.0.0.1:4000/v1/models
"endpoint": "http://127.0.0.1:11434/v1" -> http://127.0.0.1:11434/v1/models
```

A trailing `/v1` is accepted and not duplicated, since agent base URLs are
commonly written that way.

For a server that serves its model list somewhere else — a gateway under a path
prefix, or a non-standard API — give the full URL with `modelURL` instead. It
overrides `endpoint` when both are set:

```json
"modelURL": "https://gateway.example.com/openai/v1/models"
```

Omitting both skips model fetching entirely: the agent just launches with its
configured args and environment.

The model list is fetched with a 10-second timeout and parsed as either:

- **OpenAI-compatible** — `{"data": [{"id": "..."}]}`, as served by vLLM,
  LiteLLM, oMLX, llama.cpp and the OpenAI API itself
- **Ollama** — `{"models": [{"name": "..."}]}` from `/api/tags`

A `max_model_len` or `context_length` field on a model, where the endpoint
provides one, becomes the `{{contextWindow}}` placeholder. Ollama reports
neither, so `{{contextWindow}}` is simply unavailable there — see below.

### Using Ollama

Ollama works with every agent, including Claude Code. Since 0.34 it serves an
Anthropic-compatible `/v1/messages` alongside its native and OpenAI-compatible
APIs, so no translating proxy is needed.

**Claude Code** takes its model through the environment:

```json
"claude-ollama": {
  "executable": "claude",
  "description": "Claude Code via Ollama",
  "endpoint": "http://127.0.0.1:11434",
  "modelEnvVars": [
    "ANTHROPIC_DEFAULT_OPUS_MODEL",
    "ANTHROPIC_DEFAULT_SONNET_MODEL",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL"
  ],
  "envVars": {
    "ANTHROPIC_BASE_URL": "http://127.0.0.1:11434",
    "ANTHROPIC_AUTH_TOKEN": "ollama"
  }
}
```

**OpenAI-speaking agents** (codex, opencode, pi) use the `/v1` endpoint. How you
point them there is agent-specific — codex ignores `OPENAI_BASE_URL` and selects
a provider instead, and it ships `ollama` as a built-in one:

```json
"codex-ollama": {
  "executable": "codex",
  "args": ["-c", "model_provider=ollama"],
  "description": "Codex via Ollama",
  "endpoint": "http://127.0.0.1:11434",
  "modelArgs": ["-m", "{{model}}"]
}
```

Notes:

- `endpoint` is enough: Ollama serves `/v1/models` alongside its native
  `/api/tags`, and both list the same models.
- No `modelAuthToken`: listing models needs no auth. The `ANTHROPIC_AUTH_TOKEN`
  above is what Claude Code sends to `/v1/messages`, which does require a bearer
  token — any non-empty value works.
- Ollama reports no context window, so `{{contextWindow}}` has no value and any
  argument referencing it is dropped. Harmless to leave in.
- Ollama model names contain colons (`qwen3-coder-next:latest`). They pass
  through intact; if you add a provider prefix, check the agent parses the result.
- Setting a base-URL environment variable is not enough for every agent. Codex
  resolves its endpoint through `model_provider` in `~/.codex/config.toml` and
  ignores `OPENAI_BASE_URL` entirely; `ollama` is a reserved built-in provider
  id there, so select it rather than defining your own.
- Ollama has its own `ollama launch <integration>` covering many of the same
  agents. `hlw` differs in working against any endpoint, offering the interactive
  picker, and keeping per-harness args.

### Reusing the endpoint in envVars

`endpoint` tells `hlw` where to fetch the model list. It is **not** passed to the
agent — agents each read their own base-URL variable, so you still have to set
one. To avoid writing the URL twice, a value in `envVars` may contain
`{{endpoint}}`:

```json
"claude": {
  "executable": "claude",
  "endpoint": "http://127.0.0.1:4000",
  "envVars": {
    "ANTHROPIC_BASE_URL": "{{endpoint}}"
  }
}
```

It substitutes inside a larger string, so an agent wanting the `/v1` suffix works
too:

```json
"envVars": { "OPENAI_BASE_URL": "{{endpoint}}/v1" }
```

A trailing slash on `endpoint` is trimmed, so `{{endpoint}}/v1` never doubles up.
Using it without an `endpoint` set is rejected when the config loads.

### Pulling a value from another environment variable

`{{env:NAME}}` is replaced by the value of `NAME` in the process environment.
That keeps a secret out of the config file even when the variable the agent
wants has a different name from the one you export:

```bash
export OMLX_API_KEY="sk-..."      # what you export
```

```json
"envVars": {
  "ANTHROPIC_AUTH_TOKEN": "{{env:OMLX_API_KEY}}"
}
```

If `NAME` is unset or empty the entry is **not set at all**, and `hlw` says so
rather than exporting the literal placeholder. The agent then falls back to
whatever it inherits from your environment.

`{{endpoint}}` and `{{env:NAME}}` are the only placeholders available in
`envVars` — the selected model is not known at that point, and `modelEnvVars`
covers that case. Shell-style `$VAR` is **not** expanded, and a mistyped
`((name))` is rejected at load time rather than exported verbatim.

### Keeping secrets out of the config file

`modelAuthToken` is the bearer token `hlw` sends when fetching the model list.
It takes a **value**, and supports `{{env:NAME}}` — so a token you already
export in your shell never has to be copied into this file:

```bash
# ~/.zshrc
export OMLX_API_KEY="sk-..."
```

```json
"codex": {
  "executable": "codex",
  "endpoint": "http://127.0.0.1:4000",
  "modelAuthToken": "{{env:OMLX_API_KEY}}",
  "modelArgs": ["-m", "{{model}}"]
}
```

This is separate from `envVars`, which is what the **agent** receives. The two
are independent: `modelAuthToken` authenticates `hlw`'s own request, while the
agent authenticates itself with whatever its own variable holds.

The agent subprocess inherits your environment, so an exported variable reaches
it either way — `envVars` is only needed for values you want to *set or override*
per harness. If the named variable is missing from both places, the model list is
fetched unauthenticated and you get a warning saying so.

There is **no implicit token lookup**. Without `modelAuthToken` the model list is
fetched unauthenticated, whatever `envVars` happens to contain — so a harness
pointed at an arbitrary endpoint can never send a token you did not give it.

### Passing the selected model to the agent

Agents disagree about how they take a model, so each harness declares its own
wiring. Both `modelEnvVars` and `modelArgs` require `endpoint` or `modelURL` —
without a model list there is nothing to select.

**`modelEnvVars`** sets each named environment variable to the selected model id.
Claude Code works this way:

```json
"modelEnvVars": [
  "ANTHROPIC_DEFAULT_OPUS_MODEL",
  "ANTHROPIC_DEFAULT_SONNET_MODEL",
  "ANTHROPIC_DEFAULT_HAIKU_MODEL"
]
```

**`modelArgs`** appends arguments after `args`, substituting placeholders from
the selected model. Codex, OpenCode and Pi all take their model as a flag:

```json
"modelArgs": ["-m", "{{model}}"]                 // codex -m <model>
"modelArgs": ["--model", "myprovider/{{model}}"] // opencode / pi <provider>/<model>
```

| Placeholder | Value |
|-------------|-------|
| `{{model}}` | The selected model's id |
| `{{contextWindow}}` | Its context window in tokens, where the endpoint reports one |

Placeholders substitute inside a larger string, so provider prefixes work. An
argument naming a placeholder with no value is **dropped whole**, so a
half-substituted flag never reaches the agent — write flag and value as a single
token so the drop is clean:

```json
"modelArgs": ["-m", "{{model}}", "--config=model_context_window={{contextWindow}}"]
```

If no model is selected — no endpoint configured, or the fetch failed — all
`modelArgs` are dropped. Unknown placeholder names are rejected when the config loads.

The final command line is `args` + `modelArgs` + your own arguments.

## Usage

### List configured harnesses

```bash
hlw list
```

```
Configured harnesses (8):

  claude               claude
    Args:      [--permission-mode bypassPermissions --disallowedTools LSP]
    EnvVars:   6
    Models:    http://127.0.0.1:4000/v1/models
    ModelEnv:  [ANTHROPIC_DEFAULT_OPUS_MODEL ANTHROPIC_DEFAULT_SONNET_MODEL ANTHROPIC_DEFAULT_HAIKU_MODEL]
    Description: Claude Code

  claude-ollama        claude
    Args:      [--permission-mode bypassPermissions]
    EnvVars:   2
    Models:    http://127.0.0.1:11434/v1/models
    ModelEnv:  [ANTHROPIC_DEFAULT_OPUS_MODEL ANTHROPIC_DEFAULT_SONNET_MODEL ANTHROPIC_DEFAULT_HAIKU_MODEL]
    Description: Claude Code via Ollama

  codex                codex
    Models:    http://127.0.0.1:4000/v1/models
    ModelArgs: [-m {{model}} --config=model_context_window={{contextWindow}}]
    Description: OpenAI Codex

  codex-ollama         codex
    Args:      [-c model_provider=ollama]
    Models:    http://127.0.0.1:11434/v1/models
    ModelArgs: [-m {{model}}]
    Description: Codex via Ollama

  opencode             opencode
    Models:    http://127.0.0.1:4000/v1/models
    ModelArgs: [--model omlx/{{model}}]
    Description: OpenCode

  opencode-ollama      opencode
    Models:    http://127.0.0.1:11434/v1/models
    ModelArgs: [--model ollama/{{model}}]
    Description: OpenCode via Ollama

  pi                   pi
    Models:    http://127.0.0.1:4000/v1/models
    ModelArgs: [--model omlx/{{model}}]
    Description: Pi

  pi-ollama            pi
    Models:    http://127.0.0.1:11434/v1/models
    ModelArgs: [--model ollama/{{model}}]
    Description: Pi via Ollama
```

### Launch an agent

```bash
hlw launch claude
```

`hlw` will:

1. Load the harness configuration
2. Fetch the model list from `endpoint` (or `modelURL`), if one is configured
3. Show a scrolling picker so you can choose one, unless you named a model
4. Exec the agent with `args` + `modelArgs` + your own arguments, and the
   configured environment

Extra arguments are appended after the harness's own `args`:

```bash
hlw launch claude --resume xxxx
```

### Naming the model instead of picking it

Passing a model id skips the picker:

```bash
hlw launch claude deepseek-v3.2
hlw launch claude deepseek-v3.2 --resume xxxx
```

Only an argument that matches a model the endpoint actually serves is treated
this way. Anything else is passed through to the agent untouched, so agent
subcommands and flags keep working:

```bash
hlw launch opencode run "fix the bug"   # "run" is an opencode subcommand
hlw launch claude --resume xxxx         # flags are never read as models
```

### Shell completion

`hlw` completes harness names and, once a harness is named, the models its
endpoint serves:

```
$ hlw launch <TAB>
claude          -- Claude Code
claude-ollama   -- Claude Code via Ollama
codex           -- OpenAI Codex

$ hlw launch claude <TAB>
qwen3-coder-30b   deepseek-v3.2   llama3.3-70b   devstral-small
```

Model completion queries the endpoint, with a two-second timeout so an
unreachable server offers nothing rather than stalling the prompt.

Install it for your shell:

```bash
# zsh — with a directory already on $fpath
hlw completion zsh > "${fpath[1]}/_hlw"

# bash
hlw completion bash > /usr/local/etc/bash_completion.d/hlw

# fish
hlw completion fish > ~/.config/fish/completions/hlw.fish
```

`hlw completion --help` covers the details for each shell.

### The model picker

```
Select a model (18 available)

  qwen3-coder-30b
  llama-3.3-70b
> mistral-large
  devstral-small
  command-r-plus
  gemma-3-27b
  phi-4 ▼

↑/↓ navigate • pgup/pgdn page • / filter • enter select • q cancel
```

| Key | Action |
|-----|--------|
| `↑`/`k`, `↓`/`j` | Move the cursor |
| `pgup`/`b`, `pgdn`/`f` | Page up/down |
| `home`/`g`, `end`/`G` | Jump to first/last |
| `/` | Filter by name (case-insensitive); `esc` clears it |
| `enter` | Select |
| `q`, `esc`, `ctrl+c` | Cancel |

The list scrolls when it is longer than the terminal, with `▲`/`▼` marking more
models above or below. Cancelling exits with status 130 without launching.

When stdin or stdout is not a terminal — piped input, CI, a dumb terminal — it
falls back to a numbered prompt automatically, so the tool stays scriptable:

```bash
echo 3 | hlw launch claude
```

A harness whose endpoint serves exactly one model skips the prompt entirely.

### Show version

```bash
hlw --version
```

## Agent-specific notes

These are quirks of the agents themselves, not of `hlw`, but they will bite you
when wiring up a harness.

**Provider prefixes.** OpenCode and Pi name models as `<provider>/<model>`,
resolved against a provider defined in *their* config —
`~/.config/opencode/opencode.json` and `~/.pi/agent/models.json`. `hlw` does not
write those files; define the provider there first and match the prefix in your
`modelArgs`. Codex needs no prefix when `model_provider` is already set in
`~/.codex/config.toml`. To check what each will accept: `opencode models`,
`pi --list-models`.

**Pi's model catalog.** Pi only accepts models listed in its `models.json`. Some
tools rewrite that file on launch and collapse it to a single entry, after which
other models fall back to a "custom model id" with no context-window metadata.
If you switch models often, keep the full list in that file.

**Codex metadata warning.** Codex prints `Model metadata for <model> not found.
Defaulting to fallback metadata` for any model outside its built-in catalog.
Passing `--config=model_context_window={{contextWindow}}` does not silence the
text, but it *is* honoured — codex uses the real context window rather than a
small default, which is the part that actually affects behaviour. Silencing the
warning itself requires codex's `model_catalog_json`, whose schema is large,
nested, undocumented, and includes the agent's entire system prompt — pinning it
is more likely to cause drift than to help.

## What hlw does not do

- It does not write agent configuration files. Provider definitions live in each
  agent's own config.
- It does not proxy or intercept API traffic.
- It does not manage, download, or serve models.
- It does not store credentials. Tokens come from your config file or your
  environment and are passed straight to the agent.

## Development

```bash
make build      # build ./hlw for this machine
make test       # go test -race ./...
make check      # fmt + vet + test, what CI runs
make dist       # cross-compiled tarballs + checksums into dist/
make clean      # remove dist/ and ./hlw
```

### Branching and releases

Two workflows, and neither runs on a plain branch push:

| | trigger | does |
|---|---|---|
| [`ci.yml`](.github/workflows/ci.yml) | pull request to `main` | gofmt, `go vet`, `go mod tidy` check, `go test -race`, validates `config.example.json`, then builds a **release candidate** and attaches it to the run as `hlw-<version>-rc.<run>` |
| [`release.yml`](.github/workflows/release.yml) | push to `main` | re-runs the checks, then builds, tags `v<VERSION>` and publishes a GitHub release with all four binaries and checksums |

The version lives in [`VERSION`](VERSION). The release job is idempotent: if
`v<VERSION>` already exists it builds and stops, so ordinary merges do not cut a
release. **To release, bump `VERSION` in the PR** — merging it then tags and
publishes.

Add the matching section to [`CHANGELOG.md`](CHANGELOG.md) in the same PR: the
release job uses that version's entry as the GitHub release notes, and falls
back to generated notes only when the section is missing. After a release the
job renders `Formula/hlw.rb.template` with the new tag and checksum and pushes
it to the [tap repository](https://github.com/first-it-consulting/homebrew-tap).
That uses a `TAP_DEPLOY_KEY` secret holding the private half of a write-enabled
[deploy key](https://docs.github.com/en/authentication/connecting-to-github-with-ssh/managing-deploy-keys)
on the tap — deliberately not a personal access token, since a deploy key can
reach that one repository and nothing else. Without the secret the release
still succeeds and the job summary prints the formula to copy across by hand.

### Project structure

```
hlw/
├── cmd/
│   ├── root.go         # Root command and version info
│   ├── list.go         # List harnesses command
│   └── launch.go       # Launch agent command
├── internal/
│   ├── config/         # Config loading, validation, model wiring
│   ├── harness/        # Subprocess launch logic
│   └── models/
│       ├── models.go   # Model list fetching
│       ├── select.go   # Selection entry point + non-TTY fallback
│       └── picker.go   # Interactive scrolling picker
├── Formula/            # Homebrew formula template, rendered to the tap
├── CHANGELOG.md        # Release notes, per version
├── config.example.json # Example configuration
├── config.schema.json  # JSON schema for config validation
└── main.go             # Entry point
```

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT — see [LICENSE](LICENSE).

## Acknowledgments

- [Cobra](https://github.com/spf13/cobra) for the CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
  [Lip Gloss](https://github.com/charmbracelet/lipgloss) for the interactive
  model picker
