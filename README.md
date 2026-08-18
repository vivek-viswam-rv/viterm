# viterm

viterm is a terminal workspace manager for AI coding agents. It runs in your
terminal as a tmux-style multiplexer built around one idea: every piece of
work gets its own git worktree, its own branch, and its own arrangement of
panes running the tools you need, with the agent front and center.

A session is a git worktree. Opening one gives you a split layout of real
terminal panes: typically the `claude` CLI on one side and a shell on the
other, defined per repository in a small layout language. Claude Code hooks
tint the session tab as the agent works, so you can run several sessions in
parallel and see at a glance which one finished and which one is waiting for
your approval. Reattaching a session resumes the agent conversation where it
left off.

## Features

- Sessions backed by git worktrees: create a session and viterm branches
  from the default branch into an isolated worktree, so parallel work never
  collides.
- Split panes and tabs: columns and rows of PTY-backed terminals, declared
  per repository in a layout file, with vim-style pane navigation, maximize,
  and a scrollback copy mode.
- Agent status at a glance: installed hooks tint each session tab green when
  the agent finishes, red when it waits for permission, and clear it when a
  new prompt is submitted.
- Conversation resume: viterm records the agent conversation ID and appends
  `--resume` when you reattach a session, so context is never lost.
- Scriptable from inside panes: the same binary acts as a client over a
  per-session socket, so shell commands and hooks can list tabs, send text,
  start commands, open URLs, and tint the session tab.
- Status footer: branch, uncommitted diff size, commit, and the pull request
  resolved through the GitHub CLI, one keystroke away from opening in the
  browser.
- Single static binary for macOS and Linux with no runtime dependencies.

## Installation

On macOS, download `viterm-<version>.pkg` from the releases page and open
it. The installer places a universal binary at `/usr/local/bin/viterm`. If
macOS reports that the package is from an unidentified developer, control-
click the file and choose Open.

With Go installed:

```
go install github.com/vivekviswam/viterm/cmd/viterm@latest
```

Or download a binary archive from the releases page and place it on your
PATH. A local installer can be built from a checkout with
`./scripts/build-pkg.sh <version>`.

Requirements: `git`. Optional: `gh` (GitHub CLI) for pull request status,
and the `claude` CLI for agent sessions.

## Getting started

1. Run `viterm`. The setup screen opens on first launch.
2. Register a repository by entering the path of a local clone.
3. Create a session: pick the repository, type a session name, and viterm
   creates a worktree and opens the layout.
4. Install the agent hooks once with `viterm install-hooks` so session tabs
   reflect agent activity.

All commands start with the prefix key, `ctrl+space` by default:

| Keys | Action |
| --- | --- |
| `c` / `x` | new / close tab |
| `n` `p` `1`-`9` | switch tab |
| `v` / `s` | split right / down |
| `h` `j` `k` `l` | focus pane by direction |
| `z` | maximize or restore pane |
| `[` | scrollback copy mode |
| `g` / `G` | open / close the diff view |
| `N` | new session |
| `d` / `D` | detach / delete session |
| `tab` / `shift+tab` | switch session |
| `o` | open the pull request or commit page |
| `?` | help |
| `Q` | quit |

Press the prefix twice to send it to the program in the pane. Every other
keystroke goes straight to the pane, so full-screen programs behave exactly
as they would in a plain terminal.

## The companion CLI

Inside any pane, `VITERM_SOCKET` and `VITERM_PANE_ID` are set and the
`viterm` binary acts as a client of the running session:

```
viterm tabs                 # list tabs in this session
viterm send 3 'make test'   # type text into tab 3
viterm run 'npm run dev'    # open a new terminal tab running a command
viterm visit http://localhost:3000   # open a URL in the browser
viterm notify red           # tint this session's tab
viterm install-hooks        # install the agent status hooks (idempotent)
```

This is how tooling drives the workspace: a dev server can open its own URL
when it boots, and agent hooks report progress without any polling.

## Configuration

Configuration lives in `~/.config/viterm/`:

- `settings.json`: worktree base directory (default
  `~/code/viterm-worktrees`), diff command (default `git diff`), a command to
  run after each worktree is created (receives `WORKTREE_DIRECTORY`), and the
  prefix key.
- `repos.json`: registered repositories and their layouts.
- `sessions.json`: session records, including detached sessions that can be
  reattached later.

The layout language is documented in [docs/layout.md](docs/layout.md).

## Building from source

```
go build ./cmd/viterm
go test ./...
```

Releases are built with goreleaser for darwin/amd64, darwin/arm64,
linux/amd64, and linux/arm64.

## License

MIT. See [LICENSE](LICENSE).
