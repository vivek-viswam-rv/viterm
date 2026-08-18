# Layout language

Each repository registered with viterm carries a layout describing how new
sessions arrange their panes and what each pane runs. The format is
line-based. Every line has the form `key: value`. Keys are case-insensitive;
values keep their case. Blank lines are ignored. Indentation is accepted for
readability but carries no structural meaning: the tree is built from the
order of the lines alone.

## Keys

| Key | Meaning |
| --- | --- |
| `run: CMD` | A terminal pane. `CMD` is typed into a login shell started in the session worktree. An empty value opens a plain shell. |
| `visit: URL` | A link pane showing `URL`. Selecting it opens the URL in the system browser. |
| `split: columns` | Splits the current slot into two side-by-side children. Any value other than the exact word `columns` splits into stacked rows instead. |
| `left:` `right:` `top:` `bottom:` | Optional labels before each side of a split. They are documentation only; the first child parsed is always the left or top side. |
| `size: N%` | Optional, before a split child: that side's share of the split. When only the second side has a size, the first side receives the remainder. Shares are clamped between 5% and 95%. A value that fails to parse is ignored. |
| `tabs:` | A single pane whose tabs are the `run:` and `visit:` lines that immediately follow. The list ends at the first line with any other key. |

Unknown keys are skipped. A split must produce two children; if the input
ends before the second child completes, the whole split is dropped.

## Default layout

```
split: columns
left:
  size: 60%
  run: claude
right:
  run:
```

This opens the agent in a wide left pane and a plain shell on the right.

## Examples

Three stacked panes, agent on top:

```
split: rows
top:
  size: 70%
  run: claude
bottom:
  split: columns
  left:
    run:
  right:
    run: npm run dev
```

One pane with several tabs:

```
tabs:
run: claude
run: npm run dev
visit: http://localhost:3000
```

## Resuming agent conversations

When a session is reattached, any `run:` command whose first word is exactly
`claude` is extended automatically: with a recorded conversation ID whose
transcript still exists, `--resume <id>` is appended; otherwise, if the
worktree has any prior transcripts, `--continue` is appended. Commands that
already contain `--resume` or `--continue` are left untouched.
