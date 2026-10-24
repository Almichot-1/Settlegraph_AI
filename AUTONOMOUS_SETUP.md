# SettleGraph Autonomous Codex Runner (v1)

**Purpose:** Run through the SettleGraph SG-001..SG-022 backlog on your own Linux laptop without manually typing "continue." The controller launches a fresh build session and an independent read-only review session for each ticket, runs local validation, and records reviewed commits. It does **not** mean the backend has been built; it starts working only when you execute the commands below on your own machine.

## Preconditions

- Linux/Omarchy, Git, Python 3.10+, Go and Docker Compose (for database integration), `codex` CLI authenticated and available on PATH. `tmux` optional.
- Install PyYAML and jsonschema used by pack validation, e.g. inside your normal Python environment: `python -m pip install pyyaml jsonschema`.
- Choose an isolated project directory. **Full access means Codex can read/modify files outside the project and invoke networked commands despite the prompt; the controller cannot technically prevent that.** Running it on your main laptop is your decision, not a security boundary. Prefer a disposable Linux user/VM/container with only the required repositories and minimal credentials.
- `--unrestricted` uses `codex exec --dangerously-bypass-approvals-and-sandbox`. **This is intentionally dangerous**, not required for ordinary development. Default mode is `--sandbox workspace-write --ask-for-approval never`.
- No live bank data. Cloud deployment is blocked unless the separate `--allow-cloud` switch is passed. Even without this switch, an unsandboxed agent could technically access existing cloud credentials; log out/remove them before running if necessary.

## Installation

1. Download `SettleGraph_Autonomous_Builder_Pack.zip` and unpack it in `~/Projects`:

   ```bash
   mkdir -p ~/Projects
   unzip ~/Downloads/SettleGraph_Autonomous_Builder_Pack.zip -d ~/Projects
   cd ~/Projects/SettleGraph_Backend_Execution_Pack
   ```

2. Confirm the CLI and tools:

   ```bash
   command -v codex && codex exec --help | head -30
   go version && git --version && docker compose version
   python -m venv .venv
   source .venv/bin/activate
   python -m pip install pyyaml jsonschema
   python validation/validate_pack.py
   ```

3. Create an initial Git baseline. This is necessary for independent diff reviews and automatic commits:

   ```bash
   git init
   git add .
   git commit -m 'Initial SettleGraph engineering specifications'
   ```

4. Check the exact next task (does not start the AI):

   ```bash
   python autonomous_build.py --dry-run --unrestricted
   ```

5. Start the controller. **This gives Codex unrestricted OS access**:

   ```bash
   python autonomous_build.py --unrestricted --max-tickets 22
   ```

   Recommended alternative (no unrestricted host access):

   ```bash
   python autonomous_build.py --max-tickets 22
   ```

## Run while you work elsewhere

Use `tmux` to keep the terminal session running:

```bash
tmux new -s settlegraph
cd ~/Projects/SettleGraph_Backend_Execution_Pack
python autonomous_build.py --unrestricted --max-tickets 22
```

Press `Ctrl+B` then `D` to detach. Reconnect with `tmux attach -t settlegraph`. To observe another terminal:

```bash
cd ~/Projects/SettleGraph_Backend_Execution_Pack
cat .autonomous-codex/runner-status.json
ls .autonomous-codex/*.completion.json
tail -n 40 .autonomous-codex/SG-001.attempt-1.log
```

To request stop **after the current ticket/process step**:

```bash
touch .autonomous-codex/STOP
```

To resume when safe, inspect `git status`, remove the STOP file, and run:

```bash
rm .autonomous-codex/STOP
python autonomous_build.py --unrestricted --resume --max-tickets 22
```

If the controller is active, a second instance refuses to start. If a ticket becomes BLOCKED, inspect its log, reviewer verdict, tests, and uncommitted changes before resuming. Do not use `git reset --hard` blindly.

## Controller behavior

1. Reads `AGENT_BACKLOG.json` and chooses the earliest ticket with all dependencies at `TESTED` or `ACCEPTED`.
2. Asks Codex to implement only that ticket, without changing the agreed API contract without a demonstrated reason.
3. Runs independent local `python validation/validate_pack.py`, `go test ./...`, and `go vet ./...` checks; the reviewing agent additionally inspects required ticket-specific tests.
4. Starts a separate **read-only** Codex review session. It must return structured PASS/FAIL/BLOCKED findings.
5. On FAIL, feeds review notes to another build attempt, up to the configured maximum. On PASS, marks `TESTED` (not human-accepted), commits code, and continues. On BLOCKED or repeated failures, stops and reports.
6. Requires explicit `--allow-cloud` before reaching SG-021 deployment. Cloud use may cost money, and this flag is not a budget cap.

### Limits

- Reviewers and code tests can miss bugs. A passing review is not a security audit or formal correctness proof.
- The controller does not automatically verify every Docker/database integration scenario; the implementation agent must supply those tests, and the reviewer must scrutinize them.
- The user must arrange the frontend/mobile team's acknowledgements and client integration; an agent cannot truthfully sign for a human.
- Usage caps, network failures, laptop sleep, missing credentials, missing tooling, and broken migrations can stop progress.
- The script does not persist tasks to ChatGPT and cannot execute when your laptop is powered off or suspended.
- **Unrestricted access is dangerous**: despite prompt-level restrictions, malicious repository text, flawed reasoning, or a bad command could alter arbitrary local files or access sensitive credentials.

## Troubleshooting

- `codex` exits with server draining error: check `codex exec --help`, update the CLI if needed and retry; don't use untested daemon flags.
- `git` not clean: inspect `git status`; `--resume` accepts the existing interrupted working tree, but verify changes first.
- Python imports missing: install `pyyaml` and `jsonschema` in your Python environment.
- Review fails due to unsupported `--output-schema`: update Codex CLI, or adapt the controller to parse a validated JSON response; do **not** silently skip independent review.
- `go.mod` absent after SG-001: inspect the build logs and repair the ticket; don't mark it passed.
