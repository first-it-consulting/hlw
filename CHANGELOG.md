# Changelog

All notable changes to this project are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.1] - 2026-09-13

First patch on the public release. No behaviour changes to the tool itself —
this fixes the parts around it that were broken on the way out the door.

### Fixed

- **Homebrew installation could not work as documented.** The README said
  `brew tap first-it-consulting/hlw`, but Homebrew derives a tap's repository
  name by prepending `homebrew-`, so it looked for `first-it-consulting/homebrew-hlw`
  — a repository that does not exist. A `Formula/` directory in the main repo
  does not make it a tap. The instructions now point at
  [`first-it-consulting/homebrew-tap`](https://github.com/first-it-consulting/homebrew-tap),
  and explain the naming rule so the change is not reverted later.
- **The release workflow failed when updating the formula.** The step used
  `sed -i ''`, which is BSD/macOS syntax; on the GNU sed the Ubuntu runner
  ships, the empty argument is consumed as the script and the real expression
  is then treated as a filename. Both the `v0.1.1` and `v0.2.0` runs died with
  `sed: can't read s|url ...`, leaving the formula pinned at `0.1.0`.
- **Homebrew-built binaries reported `commit none`.** The formula set
  `cmd.Version` and `cmd.Date` but not `cmd.Commit`. It now stamps the tag it
  was built from, which is the most precise identifier available from a source
  tarball.

### Changed

- The release workflow now mirrors `Formula/hlw.rb` into the tap repository
  after a successful release, so the tap no longer drifts behind the latest
  tag. This needs a `TAP_TOKEN` secret with `contents: write` on the tap repo;
  without it the release still succeeds and the job summary says the tap was
  left untouched.
- Fetching the source tarball checksum now retries, since GitHub takes a
  moment to generate the archive after a tag is pushed.

### Added

- This changelog.

> **Note:** a `v0.2.0` tag was published briefly and withdrawn. It contained
> only the documentation and CI changes listed above — no feature work — so the
> release was re-cut as a patch. Nothing had been downloaded from it.

## [0.1.0] - 2026-09-13

Initial public release.

### Added

- `hlw launch <harness>` — start a configured coding agent, choosing the model
  at launch time.
- `hlw list` — show the configured harnesses.
- Model discovery against OpenAI-compatible endpoints (`/v1/models`) and Ollama
  (`/api/tags`), including context-window detection via `max_model_len` and
  `context_length`.
- An interactive scrolling model picker, with a non-TTY fallback.
- Naming a model directly — `hlw launch claude <model>` skips the picker, while
  anything that is not a known model is passed through to the agent untouched.
- Shell completion for bash, zsh and fish: completing the harness name offers
  the configured harnesses with their descriptions, and the next word offers
  the models that harness's endpoint actually serves.
- Configuration placeholders: `{{model}}`, `{{contextWindow}}`, `{{endpoint}}`
  and `{{env:NAME}}`, so API keys can stay in the environment rather than in
  the config file.
- Secret masking in the launch banner for anything that looks like a token,
  key, secret, password or credential.
- Pass-through of user arguments and flags to the underlying agent.
- A JSON schema for the configuration file, and a validated example config.

[Unreleased]: https://github.com/first-it-consulting/hlw/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/first-it-consulting/hlw/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/first-it-consulting/hlw/releases/tag/v0.1.0
