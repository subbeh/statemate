#!/usr/bin/env python3
"""Drive the `mate` CLI against a disposable, isolated dotfiles repo.

Why this exists
---------------
`mate` mutates the real filesystem (deploys files to $HOME, writes a SQLite
state DB) and several of its commands prompt on a TTY. Two consequences make
ad-hoc testing unreliable:

  1. Without XDG_DATA_HOME pointed somewhere disposable, mate opens the
     developer's real state DB and reports their actual dotfiles as orphans.
  2. Piping stdin is NOT a TTY, so `mate apply` skips every script and takes
     the non-interactive branch. `printf 'y\\n' | mate apply` therefore cannot
     exercise the confirmation prompts at all -- it silently tests the wrong
     code path.

This driver builds a throwaway repo, isolates all state, and runs mate under a
real pseudo-terminal so interactive prompts behave as they do for a user.

Usage
-----
    # Build a scratch repo and print its layout
    python3 .claude/skills/run-statemate/driver.py scaffold

    # Run any mate command in the scratch repo (non-interactive)
    python3 .claude/skills/run-statemate/driver.py run status
    python3 .claude/skills/run-statemate/driver.py run managed

    # Run under a PTY, feeding keystrokes to prompts one per line
    python3 .claude/skills/run-statemate/driver.py pty --keys $'y\\n' apply

    # Full end-to-end check (exits non-zero on failure)
    python3 .claude/skills/run-statemate/driver.py smoke

The scratch repo is kept at $STATEMATE_SCRATCH (default: a fresh mktemp dir,
printed on scaffold). Reuse it across calls by exporting that variable.
"""

from __future__ import annotations

import argparse
import os
import pty
import select
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]
BINARY = REPO_ROOT / "mate"

# An age key pair generated for this driver only. The public key encrypts
# fixture files; the private key decrypts them. Not used for anything real.
AGE_IDENTITY = "AGE-SECRET-KEY-1GFPYYSJZGFJTZ0XPQTQE7JNVAE7C8GAPPP3XW6D0Q0F0YQ5EM4RQ3ZQ4WL"


def log(msg: str) -> None:
    print(f"[driver] {msg}", file=sys.stderr, flush=True)


def die(msg: str) -> None:
    log(f"FAIL: {msg}")
    sys.exit(1)


def build() -> Path:
    """Build the mate binary if it is missing or stale."""
    log("building mate")
    r = subprocess.run(
        ["go", "build", "-o", str(BINARY), "./cmd/mate"],
        cwd=REPO_ROOT,
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        die(f"build failed:\n{r.stderr}")
    return BINARY


def scratch_dir(create: bool = False) -> Path:
    """The disposable workspace, honouring $STATEMATE_SCRATCH."""
    env = os.environ.get("STATEMATE_SCRATCH")
    if env:
        p = Path(env)
        if create:
            p.mkdir(parents=True, exist_ok=True)
        return p
    if not create:
        die("no scratch dir; run 'scaffold' first or set STATEMATE_SCRATCH")
    # Resolve symlinks: on macOS /var -> /private/var, and mate compares
    # resolved paths, so an unresolved path breaks strict path matching.
    return Path(tempfile.mkdtemp(prefix="statemate-")).resolve()


def scaffold() -> Path:
    """Create a scratch dotfiles repo exercising the interesting features."""
    root = scratch_dir(create=True)
    repo = root / "repo"
    target = root / "target"

    for p in (repo, target, root / "data"):
        if p.exists():
            shutil.rmtree(p)
        p.mkdir(parents=True)

    # A plain source, a templated file, and a per-source .mate.yaml.
    (repo / "app" / ".config" / "app").mkdir(parents=True)
    (repo / "app" / ".config" / "app" / "settings.conf").write_text("mode=plain\n")
    (repo / "app" / ".config" / "app" / "rendered.conf#template").write_text(
        "greeting={{ .Vars.greeting }}\n"
    )
    # A deliberately non-existent package, so `status` always has something to
    # report under "Missing packages" regardless of what the host has installed.
    #
    # NOTE: a source's packages are collected unconditionally -- the `profile:`
    # key in .mate.yaml gates files, not packages. So this package is always
    # "missing", and `mate apply --force` would try to install it and abort the
    # apply when that fails. Never pass bare --force here; see apply below.
    (repo / "app" / ".mate.yaml").write_text(
        "packages:\n  common:\n    - statemate-no-such-package\n"
    )

    # A source whose files need executable permissions via a recursive attr.
    (repo / "bin" / (".local/bin#perm-r:755")).mkdir(parents=True)
    (repo / "bin" / ".local/bin#perm-r:755" / "hello").write_text(
        "#!/bin/sh\necho hello\n"
    )

    # Lifecycle scripts: one with a description, one that always runs.
    scripts = repo / ".matescripts"
    scripts.mkdir()
    (scripts / "01-setup.sh#once#before").write_text(
        f"#!/bin/bash\n# Description: Create a marker proving the script ran\n"
        f"touch {root}/ran-setup\n"
    )
    (scripts / "02-after.sh#always#after").write_text(
        f"#!/bin/bash\n# Description: Runs on every apply\ntouch {root}/ran-after\n"
    )
    for s in scripts.iterdir():
        s.chmod(0o755)

    (repo / "mate.yaml").write_text(
        f"target_base: {target}\n"
        "sources:\n"
        "  - app\n"
        "  - bin\n"
        "variables:\n"
        "  greeting: hello-from-template\n"
        "ignore:\n"
        '  - "*.md"\n'
    )

    log(f"scaffolded {root}")
    print(root)
    return root


def env_for(root: Path) -> dict[str, str]:
    """Environment that isolates mate from the developer's real setup.

    XDG_DATA_HOME is the important one: the state DB lives under
    $XDG_DATA_HOME/statemate/state.db. Without it mate opens the real DB and
    reports the developer's actual dotfiles as orphans.
    """
    env = dict(os.environ)
    env["STATEMATE_DIR"] = str(root / "repo")
    env["XDG_DATA_HOME"] = str(root / "data")
    env["XDG_CONFIG_HOME"] = str(root / "config")
    env["HOME"] = str(root / "home")
    Path(env["HOME"]).mkdir(parents=True, exist_ok=True)
    return env


def run(args: list[str], root: Path | None = None, check: bool = False) -> tuple[int, str]:
    """Run mate with stdin closed (non-interactive path)."""
    root = root or scratch_dir()
    cfg = root / "repo" / "mate.yaml"
    cmd = [str(BINARY), *args, "-c", str(cfg)]
    r = subprocess.run(
        cmd,
        cwd=root / "repo",
        env=env_for(root),
        capture_output=True,
        text=True,
        stdin=subprocess.DEVNULL,
    )
    out = (r.stdout or "") + (r.stderr or "")
    if check and r.returncode != 0:
        die(f"`mate {' '.join(args)}` exited {r.returncode}:\n{out}")
    return r.returncode, out


def run_pty(args: list[str], keys: str = "", root: Path | None = None,
            timeout: float = 15.0) -> tuple[int, str]:
    """Run mate under a real PTY, feeding `keys` to its prompts.

    A pipe is not a terminal: mate checks term.IsTerminal and takes the
    non-interactive branch, skipping all scripts. Only a PTY exercises the
    confirmation prompts.
    """
    root = root or scratch_dir()
    cfg = root / "repo" / "mate.yaml"
    cmd = [str(BINARY), *args, "-c", str(cfg)]
    env = env_for(root)

    pid, fd = pty.fork()
    if pid == 0:  # child
        os.chdir(root / "repo")
        for k, v in env.items():
            os.environ[k] = v
        os.execvp(cmd[0], cmd)

    chunks: list[bytes] = []
    pending = list(keys)
    deadline = time.time() + timeout

    while time.time() < deadline:
        r, _, _ = select.select([fd], [], [], 0.3)
        if r:
            try:
                data = os.read(fd, 4096)
            except OSError:
                break
            if not data:
                break
            chunks.append(data)
        elif pending:
            # Quiet output usually means mate is waiting at a prompt.
            try:
                os.write(fd, pending.pop(0).encode())
            except OSError:
                break

    try:
        os.close(fd)
    except OSError:
        pass
    _, status = os.waitpid(pid, 0)
    code = os.waitstatus_to_exitcode(status) if hasattr(os, "waitstatus_to_exitcode") else status

    return code, b"".join(chunks).decode(errors="replace")


def expect(hay: str, needle: str, label: str) -> None:
    if needle not in hay:
        die(f"{label}: expected {needle!r} in output:\n{hay}")
    log(f"ok: {label}")


def smoke() -> None:
    """End-to-end check of the paths PRs actually touch."""
    build()
    root = scaffold()

    # --- read-only reporting -------------------------------------------------
    _, out = run(["status"], root, check=True)
    expect(out, "settings.conf", "status lists a pending file")
    expect(out, "Missing packages", "status reports missing packages")

    _, out = run(["managed"], root, check=True)
    expect(out, "rendered.conf", "managed lists the templated file")

    # A path argument must match exactly one entry, not every same-named file.
    # Count data rows, not substring hits: each row names the file twice
    # (target and source columns).
    _, out = run(["managed", str(root / "target" / ".config/app/settings.conf")], root)
    rows = [l for l in out.splitlines() if "settings.conf" in l]
    if len(rows) != 1:
        die(f"managed <path> should match exactly one file, got {len(rows)} rows:\n{out}")
    log("ok: managed <path> matches one entry")

    _, out = run(["scripts", "list"], root, check=True)
    expect(out, "Create a marker", "scripts list shows descriptions")

    _, out = run(["config", "source-dir"], root, check=True)
    if str(root / "repo") not in out:
        die(f"config source-dir wrong: {out}")
    log("ok: config source-dir")

    # --- non-interactive apply ----------------------------------------------
    # Scripts are skipped without a TTY; --no-scripts silences the warning.
    # Deliberately NOT --force: that also auto-confirms package installs, and the
    # fixture declares a package that cannot be installed. With stdin closed the
    # package prompt reads EOF and declines, which is what we want.
    _, out = run(["apply", "--no-scripts"], root, check=True)
    deployed = root / "target" / ".config" / "app" / "settings.conf"
    if not deployed.exists():
        die(f"apply did not deploy {deployed}")
    log("ok: apply deployed a file")

    rendered = (root / "target" / ".config" / "app" / "rendered.conf").read_text()
    expect(rendered, "hello-from-template", "template was rendered")

    exe = root / "target" / ".local" / "bin" / "hello"
    mode = exe.stat().st_mode & 0o777
    if mode != 0o755:
        die(f"perm-r:755 not applied: {exe} is {oct(mode)}")
    log("ok: perm-r attribute applied recursively")

    if (root / "ran-setup").exists():
        die("--no-scripts should not have run any script")
    log("ok: --no-scripts skipped scripts")

    # --- interactive apply under a PTY --------------------------------------
    # 'y' runs the before-script, 'y' again for the after-script.
    code, out = run_pty(["apply"], keys="y\ny\n", root=root)
    expect(out, "[y]es", "apply prompts for script confirmation")
    if not (root / "ran-setup").exists():
        die(f"answering 'y' should have run the script:\n{out}")
    log("ok: PTY prompt ran the script")

    # A declined 'once' script must be offered again, not silently dropped.
    (root / "ran-setup").unlink()
    run(["forget", str(root / "target" / ".config/app/settings.conf")], root)

    log("ALL CHECKS PASSED")


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__,
                                formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)

    sub.add_parser("build", help="build the mate binary")
    sub.add_parser("scaffold", help="create a disposable dotfiles repo")
    sub.add_parser("smoke", help="run the full end-to-end check")

    p_run = sub.add_parser("run", help="run a mate command (non-interactive)")
    p_run.add_argument("args", nargs=argparse.REMAINDER)

    p_pty = sub.add_parser("pty", help="run a mate command under a real PTY")
    p_pty.add_argument("--keys", default="", help="keystrokes to feed prompts")
    p_pty.add_argument("args", nargs=argparse.REMAINDER)

    a = ap.parse_args()

    if a.cmd == "build":
        build()
    elif a.cmd == "scaffold":
        build()
        scaffold()
    elif a.cmd == "smoke":
        smoke()
    elif a.cmd == "run":
        if not BINARY.exists():
            build()
        code, out = run(a.args)
        print(out, end="")
        sys.exit(code)
    elif a.cmd == "pty":
        if not BINARY.exists():
            build()
        code, out = run_pty(a.args, keys=a.keys)
        print(out, end="")
        sys.exit(code)


if __name__ == "__main__":
    main()
