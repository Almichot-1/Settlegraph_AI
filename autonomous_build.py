#!/usr/bin/env python3
"""Unattended SettleGraph Codex runner. Runs locally on the user's computer.

Never launches an agent without a user's explicit command. Default uses workspace sandbox.
An explicit --unrestricted opts out of Codex's sandbox and approvals.
"""
from __future__ import annotations

import argparse
import datetime as dt
import fnmatch
import json
import os
import pathlib
import re
import shutil
import signal
import subprocess
import sys
import time
from typing import Any

OK_STATUSES = {"TESTED", "ACCEPTED"}
STOP_PATTERNS = ("rate limit", "usage limit", "insufficient_quota", "exceeded your", "quota exceeded", "too many requests")
# User explicitly authorizes local execution; cloud operations remain separately opt-in.
SENSITIVE_PATTERNS = (".env", ".env.*", "*.pem", "*.key", "*.p12", "*.pfx", "*.tfstate", "*.tfstate.*", "*credentials*", "*.sqlite", "*.db")


def now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat(timespec="seconds")


def read_json(p: pathlib.Path) -> dict[str, Any]:
    return json.loads(p.read_text(encoding="utf-8"))


def write_json(p: pathlib.Path, data: dict[str, Any]) -> None:
    p.parent.mkdir(parents=True, exist_ok=True)
    tmp = p.with_suffix(p.suffix + ".tmp")
    tmp.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    tmp.replace(p)


def git(root: pathlib.Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["git", *args], cwd=root, text=True, capture_output=True, check=False)


def tracked_changes(root: pathlib.Path) -> list[str]:
    p = git(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
    if p.returncode:
        raise RuntimeError(p.stderr.strip())
    entries = p.stdout.split("\0")
    paths: list[str] = []
    # porcelain rename entries have an additional destination field
    i = 0
    while i < len(entries):
        entry = entries[i]
        if not entry:
            i += 1
            continue
        paths.append(entry[3:])
        if entry[:2] in ("R ", " R", "RR", "C ", " C") and i + 1 < len(entries):
            i += 1
            paths.append(entries[i])
        i += 1
    return paths


def sensitive_paths(paths: list[str]) -> list[str]:
    flagged = []
    for rel in paths:
        parts = pathlib.PurePosixPath(rel).parts
        for part in parts:
            lower = part.lower()
            if lower.endswith(".example") or lower.endswith(".sample") or lower.endswith(".template"):
                continue
            if any(fnmatch.fnmatch(lower, pattern) for pattern in SENSITIVE_PATTERNS):
                flagged.append(rel)
                break
    return sorted(set(flagged))


def pick_ticket(backlog: dict[str, Any]) -> dict[str, Any] | None:
    tickets = backlog["tickets"]
    done = {t["id"] for t in tickets if t.get("status") in OK_STATUSES}
    active = [t for t in tickets if t.get("status") not in OK_STATUSES and t.get("status") != "DEFERRED"]
    for t in active:
        if all(d in done for d in t.get("dependencies", [])):
            return t
    return None


def run_process(cmd: list[str], cwd: pathlib.Path, log: pathlib.Path,
                prompt: str | None, timeout: int) -> tuple[int, str]:
    log.parent.mkdir(parents=True, exist_ok=True)
    with log.open("w", encoding="utf-8") as f:
        f.write(f"# started {now()}\n# command: {cmd[0]} {cmd[1]} [arguments redacted from log header]\n")
        f.flush()
        p = subprocess.Popen(cmd, cwd=cwd, stdin=subprocess.PIPE if prompt is not None else subprocess.DEVNULL,
                             stdout=f, stderr=subprocess.STDOUT, text=True, start_new_session=True)
        try:
            p.communicate(input=prompt, timeout=timeout)
        except subprocess.TimeoutExpired:
            try:
                os.killpg(p.pid, signal.SIGTERM)
                p.wait(timeout=8)
            except (ProcessLookupError, subprocess.TimeoutExpired):
                try:
                    os.killpg(p.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                p.wait()
            f.write(f"\n# TIMEOUT after {timeout} seconds\n")
            return 124, "TIMEOUT"
        except KeyboardInterrupt:
            try:
                os.killpg(p.pid, signal.SIGTERM)
            except ProcessLookupError:
                pass
            raise
        status = p.returncode
    tail = log.read_text(encoding="utf-8", errors="replace")[-3500:]
    return status, tail


def codex_exec(root: pathlib.Path, logs: pathlib.Path, label: str, prompt: str,
               unrestricted: bool, timeout: int, review: bool = False,
               schema: pathlib.Path | None = None) -> tuple[int, str, pathlib.Path]:
    output = logs / f"{label}.answer.json" if review else logs / f"{label}.answer.md"
    log = logs / f"{label}.log"
    cmd = ["codex", "exec", "-C", str(root), "--json", "--output-last-message", str(output)]
    if review:
        cmd += ["--sandbox", "read-only", "--ask-for-approval", "never"]
        if schema:
            cmd += ["--output-schema", str(schema)]
    elif unrestricted:
        cmd += ["--dangerously-bypass-approvals-and-sandbox"]
    else:
        cmd += ["--sandbox", "workspace-write", "--ask-for-approval", "never"]
    cmd += ["-"]
    rc, tail = run_process(cmd, root, log, prompt, timeout)
    return rc, tail, output


def smoke_tests(root: pathlib.Path, logs: pathlib.Path, ticket: str, timeout: int) -> tuple[bool, list[dict[str, Any]]]:
    tests: list[list[str]] = []
    if (root / "validation/validate_pack.py").exists():
        tests.append([sys.executable, "validation/validate_pack.py"])
    if (root / "go.mod").exists():
        tests.extend([["go", "test", "./..."], ["go", "vet", "./..."]])
    elif ticket != "SG-001":
        return False, [{"command": "go test ./...", "returncode": 127, "detail": "go.mod missing"}]
    else:
        return False, [{"command": "go test ./...", "returncode": 127, "detail": "SG-001 must create go.mod"}]
    results: list[dict[str, Any]] = []
    for i, cmd in enumerate(tests):
        log = logs / f"{ticket}.check.{i}.log"
        rc, tail = run_process(cmd, root, log, None, timeout)
        results.append({"command": " ".join(cmd), "returncode": rc, "log": str(log.relative_to(root)), "tail": tail[-600:]})
        if rc != 0:
            return False, results
    return bool(results), results


def review_prompt(ticket: dict[str, Any], checks: list[dict[str, Any]], allow_cloud: bool) -> str:
    return f"""You are an independent STRICT code reviewer. You must not edit files.
Ticket: {ticket['id']} - {ticket['title']}
Dependencies: {ticket.get('dependencies')}
Description: {json.dumps(ticket.get('description'), ensure_ascii=False)}
Acceptance: {json.dumps(ticket.get('acceptance'), ensure_ascii=False)}
Read AGENTS.md, AGENT_BACKLOG.json, relevant documents, contracts and current implementation.
Examine git diff against HEAD, new untracked implementation files, actual test code, and these external controller test results:
{json.dumps(checks, indent=2)}
Decide PASS only if the acceptance conditions are substantively implemented and there is credible evidence of their tests executing. Confirm no missing tenant isolation, no direct BankSim ledger reads by SettleGraph, no AI payments mutation, and no undocumented API changes where relevant. Distinguish untested from tested. Do NOT call something passing based merely on plausibility or the builder's statements. For human-dependent acceptance like frontend sign-off, say what remains pending; automated test gates may be TESTED without claiming human acceptance if all implementation and automated gates are covered.
Cloud deployment permitted in this run: {allow_cloud}. Do not perform any external operation.
Produce only strict JSON matching the supplied schema. Use verdict PASS, FAIL, or BLOCKED, and provide concrete findings and required_fixes.
"""


def build_prompt(ticket: dict[str, Any], attempt: int, feedback: str, allow_cloud: bool) -> str:
    is_fix = attempt > 1
    intro = "Repair the previously reported defects for this ticket." if is_fix else "Implement this exact ticket fully."
    return f"""You are the autonomous SettleGraph backend implementation engineer. {intro}
Work only on the repo at the current directory. Read AGENTS.md, README.md, AGENT_BACKLOG.json, PROJECT_MANAGER_DELIVERY_PLAYBOOK.md, BACKEND_IMPLEMENTATION_BLUEPRINT.md, public and BankSim OpenAPI contracts, relevant migrations, fixtures, and test requirements. Build working code, not a plan.
Ticket ID: {ticket['id']}.
Title: {ticket['title']}.
Descriptions: {json.dumps(ticket.get('description'), ensure_ascii=False)}
Acceptance: {json.dumps(ticket.get('acceptance'), ensure_ascii=False)}
Attempt: {attempt}. Previous failures: {feedback or 'none'}.
The CONTROLLER handles backlog statuses, independent tests, reviews, and commits. DO NOT mark this ticket TESTED or ACCEPTED yourself. You may update docs/implementation-progress.md and risk log with facts. Do not modify unrelated functionality, weaken assertions, fabricate client sign-off, or change frozen APIs without updating fixtures and proving compatibility.
You may work autonomously within local project. Cloud operations allowed: {allow_cloud}. If false, do not deploy, create paid cloud resources, or use AWS credentials. NEVER use real funds, real bank credentials, or external financial transactions. Simulated only. Do not run destructive host commands, sudo, or modify files outside the project. Avoid putting secrets into files or logs. Prefer offline simulations where AI/integration isn't available and label the fallback.
Write meaningful tests for the specific acceptance criteria; run phase-specific and regression tests and fix failures. Implement all of this ticket's requirements unless demonstrably impossible; report any external blocker honestly. Finish with files changed, tests actually run with exact commands and pass/fail, and remaining issues. Do not claim acceptance or make a Git commit; the controller will review and commit.
"""


def update_status(root: pathlib.Path, ticket_id: str, status: str,
                  evidence: dict[str, Any] | None = None) -> None:
    path = root / "AGENT_BACKLOG.json"
    data = read_json(path)
    for t in data["tickets"]:
        if t["id"] == ticket_id:
            t["status"] = status
            if evidence:
                t["evidence"] = evidence
            break
    write_json(path, data)


def ensure_git_baseline(root: pathlib.Path, allow_dirty: bool = False) -> None:
    result = git(root, "rev-parse", "--show-toplevel")
    if result.returncode or pathlib.Path(result.stdout.strip()).resolve() != root:
        raise RuntimeError("The project must be a Git repo at its root: git init && git add . && git commit -m 'Baseline specifications'")
    result = git(root, "rev-parse", "HEAD")
    if result.returncode:
        raise RuntimeError("Create a baseline commit first; cannot review changes without HEAD")
    dirty = tracked_changes(root)
    if dirty and not allow_dirty:
        raise RuntimeError(f"Start from a clean Git checkout (or use --resume for a stopped run). Uncommitted paths: {dirty[:15]}")


def commit_ticket(root: pathlib.Path, ticket: dict[str, Any], results: list[dict[str, Any]],
                  review: dict[str, Any]) -> str:
    changed = tracked_changes(root)
    secrets = sensitive_paths(changed)
    if secrets:
        raise RuntimeError(f"Possible secrets/sensitive files in working tree; refusing automatic commit: {secrets}")
    update_status(root, ticket["id"], "TESTED", {
        "commit": None,
        "tests": [{"command": r["command"], "result": "PASS" if r["returncode"] == 0 else "FAIL"} for r in results],
        "runtime_verified": True,
        "review": "PASS",
        "note": "Committed SHA stored in .autonomous-codex/<ticket>.completion.json",
    })
    p = git(root, "add", "--all")
    if p.returncode:
        raise RuntimeError(p.stderr.strip())
    staged = git(root, "diff", "--cached", "--name-only")
    secret_staged = sensitive_paths(staged.stdout.splitlines())
    if secret_staged:
        git(root, "reset")
        raise RuntimeError(f"Unsafe staged path(s): {secret_staged}")
    p = git(root, "commit", "-m", f"{ticket['id']}: {ticket['title']} (reviewed)")
    if p.returncode:
        raise RuntimeError(f"git commit failed: {p.stderr.strip()}")
    head = git(root, "rev-parse", "HEAD")
    return head.stdout.strip()


def runner(args: argparse.Namespace) -> int:
    root: pathlib.Path = args.project.resolve()
    if not (root / "AGENT_BACKLOG.json").exists():
        raise RuntimeError(f"No AGENT_BACKLOG.json in {root}. Extract pack first.")
    if not shutil.which("codex") and not args.dry_run:
        raise RuntimeError("codex binary is not installed or not on PATH")
    ensure_git_baseline(root, allow_dirty=args.resume)
    backlog = read_json(root / "AGENT_BACKLOG.json")
    ticket = pick_ticket(backlog)
    if args.dry_run:
        print(f"Project: {root}\nMode: {'UNRESTRICTED' if args.unrestricted else 'workspace-write'}")
        print(f"Next ticket: {ticket['id']} - {ticket['title']}" if ticket else "All ready tickets completed or blocked")
        return 0
    logs = root / ".autonomous-codex"
    logs.mkdir(exist_ok=True)
    lock = logs / "RUNNING.lock"
    # Unix Linux lock prevents two autonomous controllers overwriting each other's changes.
    import fcntl
    with lock.open("w") as lf:
        try:
            fcntl.flock(lf, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            raise RuntimeError("Another controller is running")
        lf.write(f"pid={os.getpid()} started={now()}\n")
        lf.flush()
        completed = 0
        while completed < args.max_tickets:
            if (logs / "STOP").exists():
                print("STOP requested. Leaving current project safe; exiting.")
                break
            backlog = read_json(root / "AGENT_BACKLOG.json")
            ticket = pick_ticket(backlog)
            if not ticket:
                not_done = [t["id"] + ":" + t.get("status", "?") for t in backlog["tickets"] if t.get("status") not in OK_STATUSES]
                print("No more dependency-ready tickets. Remaining:", not_done)
                break
            tid = ticket["id"]
            if tid == "SG-021" and not args.allow_cloud:
                print("STOPPED: SG-021 needs separately authorized cloud deployment; rerun with --allow-cloud after configuring a budget and credentials.")
                write_json(logs / "runner-status.json", {"state": "CLOUD_APPROVAL_REQUIRED", "ticket": tid, "updated_at": now()})
                return 2
            print(f"[{now()}] Building {tid}: {ticket['title']}", flush=True)
            write_json(logs / "runner-status.json", {"state": "BUILDING", "ticket": tid, "completed_in_this_run": completed, "updated_at": now()})
            update_status(root, tid, "IN_PROGRESS")
            accepted = False
            feedback = ""
            for attempt in range(1, args.retries + 2):
                label = f"{tid}.attempt-{attempt}"
                rc, tail, answer = codex_exec(root, logs, label, build_prompt(ticket, attempt, feedback, args.allow_cloud),
                                               args.unrestricted, args.agent_timeout)
                if rc:
                    feedback = f"Codex session exit {rc}, details: {tail[-1000:]}"
                    if any(x in tail.lower() for x in STOP_PATTERNS):
                        print("Usage/rate limit detected; stopping without extra retries.", flush=True)
                        break
                    print(f"{tid} builder exited {rc}", flush=True)
                    continue
                passed, checks = smoke_tests(root, logs, tid, args.test_timeout)
                if not passed:
                    feedback = f"Controller tests failed: {json.dumps(checks, indent=2)}"
                    print(f"{tid} failed independent tests, attempt {attempt}", flush=True)
                    continue
                schema = pathlib.Path(__file__).resolve().parent / "review_schema.json"
                review_rc, review_tail, report_path = codex_exec(root, logs, f"{label}.review", review_prompt(ticket, checks, args.allow_cloud),
                                                                   False, args.review_timeout, True, schema)
                try:
                    review = read_json(report_path) if review_rc == 0 else {"verdict": "BLOCKED", "required_fixes": [f"Reviewer exited {review_rc}: {review_tail[-700:]}"]}
                except (ValueError, OSError):
                    review = {"verdict": "BLOCKED", "required_fixes": ["No valid structured review result"]}
                write_json(logs / f"{label}.verdict.json", review)
                if review.get("verdict") == "PASS":
                    # An external reviewer may discover a violation that needs a human; don't lie about review status.
                    sha = commit_ticket(root, ticket, checks, review)
                    write_json(logs / f"{tid}.completion.json", {"ticket": tid, "status": "TESTED", "commit": sha,
                                                                       "tests": checks, "review": review, "completed_at": now()})
                    completed += 1
                    accepted = True
                    print(f"ACCEPTED {tid} at {sha[:10]} ({completed} in this run)", flush=True)
                    break
                feedback = "Independent reviewer: " + json.dumps(review, ensure_ascii=False)
                print(f"{tid} review {review.get('verdict')} on attempt {attempt}", flush=True)
                if review.get("verdict") == "BLOCKED":
                    break
            if not accepted:
                # Leave partial code visible for user; cannot auto-accept.
                update_status(root, tid, "BLOCKED")
                write_json(logs / "runner-status.json", {"state": "BLOCKED", "ticket": tid,
                                                              "reason": feedback[-1800:], "updated_at": now()})
                print(f"STOPPED at {tid}; inspect {logs}/ and uncommitted changes.\n{feedback[-600:]}", flush=True)
                return 2
        write_json(logs / "runner-status.json", {"state": "STOPPED_OR_FINISHED", "completed_in_this_run": completed, "updated_at": now()})
        return 0


def parse() -> argparse.Namespace:
    p = argparse.ArgumentParser(description="Autonomous Codex runner for SettleGraph AI")
    p.add_argument("--project", type=pathlib.Path, default=pathlib.Path.cwd(), help="root of extracted execution pack")
    p.add_argument("--unrestricted", action="store_true", help="DANGEROUS: Codex gets unsandboxed host access")
    p.add_argument("--allow-cloud", action="store_true", help="explicit approval for cloud deployment ticket SG-021; budget/credentials are your responsibility")
    p.add_argument("--max-tickets", type=int, default=22)
    p.add_argument("--retries", type=int, default=2, help="extra repair attempts per ticket")
    p.add_argument("--agent-timeout", type=int, default=3600, help="seconds per Codex builder session")
    p.add_argument("--review-timeout", type=int, default=1200)
    p.add_argument("--test-timeout", type=int, default=900)
    p.add_argument("--dry-run", action="store_true")
    p.add_argument("--resume", action="store_true", help="resume with existing uncommitted work from a previous failed/interrupted run")
    a = p.parse_args()
    if a.retries < 0 or a.max_tickets <= 0:
        p.error("--retries must be >=0 and --max-tickets must be >0")
    return a


if __name__ == "__main__":
    try:
        sys.exit(runner(parse()))
    except KeyboardInterrupt:
        print("Interrupted by user. Check .autonomous-codex and git status.", file=sys.stderr)
        sys.exit(130)
    except Exception as exc:
        print(f"FATAL: {exc}", file=sys.stderr)
        sys.exit(3)
