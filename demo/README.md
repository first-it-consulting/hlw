# Demo

[`hlw.gif`](hlw.gif) in the project README is produced by
[`record.sh`](record.sh):

```bash
./demo/record.sh
```

It drives `hlw` in a real PTY and screenshots the terminal, so the recording is
of the actual program — the picker, the filter and the launch banner are all
real output, not a mock-up.

- [`config.json`](config.json) is the harness config used for the recording, so
  nothing from your own config appears in it.
- [`models-server.py`](models-server.py) serves a small stub `/v1/models` list,
  which keeps the recording reproducible and independent of whichever models are
  installed on the machine doing the recording.
- [`script.json`](script.json) is the keystroke script.
- Recording stops at `Launching...`, where `hlw` hands over to the agent.

Requirements: `python3` with `pyte` and `pillow`, plus `ffmpeg`.
