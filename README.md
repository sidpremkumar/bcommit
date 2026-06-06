# bcommit

A command-line tool that generates git commit messages **and pull requests** using a local LLM. It reads your staged diff (or your whole branch), sends it to a model running on [Ollama](https://ollama.com), and gives you a conventional commit message or a ready-to-open PR — no API keys, no cloud, everything stays on your machine.

## How it works

When you run `bcommit`, it:

1. Grabs your staged diff (`git diff --cached`)
2. Estimates how large the diff is and picks a processing strategy:
   - **Small diffs** get sent to the model as-is
   - **Medium diffs** go through a filtering step that strips out lock files, generated code, and other noise
   - **Large diffs** get broken down per-file, each summarized individually, then aggregated
3. Sends the processed diff to a local Ollama model
4. Returns a [Conventional Commits](https://www.conventionalcommits.org/) formatted message

If Ollama isn't running, bcommit will start it for you and shut it down when it's done. If you don't have the model downloaded yet, it'll offer to pull it.

## Installation

You'll need [Go](https://go.dev/) and [Ollama](https://ollama.com) installed.

```bash
go install github.com/sidpremkumar/bcommit/cmd/bcommit@latest
```

Or build from source:

```bash
git clone https://github.com/sidpremkumar/bcommit.git
cd bcommit
go build -o bcommit ./cmd/bcommit
```

## Usage

Stage some changes and run it:

```
$ git add -p
$ bcommit
● Analyzing staged changes...
.github/workflows/uat-deploy.yml | 16 ++++++++++++++++
 1 file changed, 16 insertions(+)
● Ollama server is not running. Starting it...
✓ Ollama server started (will shut down on exit).
● Generating commit message...

  feat(.github/workflows): add pre-create SSH keys for gcloud in UAT deployment workflow

  [Copied to clipboard]
[a]ccept  [e]dit  [r]egenerate  [q]uit: a
✓ Committed: feat(.github/workflows): add pre-create SSH keys for gcloud in UAT deployment workflow
● Shutting down Ollama server (we started it)...
✓ Ollama server stopped.
```

From the interactive prompt you can:

- **a** — accept the message and commit
- **e** — open it in your `$EDITOR` to tweak it
- **r** — throw it away and generate a new one
- **q** — quit without committing

### Flags

```
-c, --commit        Auto-commit without the interactive prompt
-p, --print         Just print the message to stdout (useful for scripting)
-m, --model string  Use a different model (e.g. qwen2.5-coder:7b)
-t, --type string   Force a commit type (feat, fix, refactor, etc.)
    --hint string   Give the model extra context about what you're doing
-b, --branch        Generate a branch name, create it, then commit onto it
-v, --verbose       Show token counts, tier selection, and timing info
```

### Examples

Skip the interactive prompt and commit directly:

```bash
bcommit -c
```

Give the model a hint about what you were working on:

```bash
bcommit --hint "migrating from REST to GraphQL"
```

Force a specific commit type:

```bash
bcommit -t fix
```

Use a larger model for more complex diffs:

```bash
bcommit -m qwen2.5-coder:7b
```

Just print the message (pipe it somewhere, use it in a script, etc.):

```bash
bcommit -p
```

## Pull requests

`bcommit pr` writes the title and body of a pull request from your branch, then opens it with the [GitHub CLI](https://cli.github.com).

It:

1. Figures out the base branch — auto-detected from `origin/HEAD`, or set it with `--base` or the `default_base` config key
2. Collects the commits and diff between the base and your current branch
3. Folds in any per-repo context you've saved (see below) and runs the diff through the same small/medium/large tiering used for commits
4. Generates a title and description with your local Ollama model
5. Pushes the branch if it has no upstream, then creates the PR via `gh pr create`

You'll need the [GitHub CLI](https://cli.github.com) installed and authenticated (`gh auth login`) — bcommit checks this up front, before doing any LLM work. The branch diff is also scanned for likely secrets, and you'll be warned before anything is created.

```
$ bcommit pr
● Analyzing main...add-pr-command (3 commit(s))
 internal/cli/pr_cmd.go      | 240 +++++++++++++++++++++++++++++
 internal/gh/gh.go           |  96 +++++++++++
 ...
● Generating PR description...

  feat: add `bcommit pr` for LLM-generated pull requests

  ## Summary
  Adds a `pr` subcommand that diffs the branch against its base,
  gathers commits, and generates a title and description...

[a]ccept  [e]dit  [r]egenerate  [q]uit: a
● Pushing branch to origin...
✓ Created PR: https://github.com/sidpremkumar/bcommit/pull/2
```

The interactive prompt works just like the commit flow:

- **a** — accept, push the branch, and open the PR
- **e** — edit the title and body in your `$EDITOR`
- **r** — throw it away and generate a new one
- **q** — quit without opening a PR

### Flags

```
    --base string   Base branch to target (default: auto-detected)
-p, --print         Print the title and body only (no PR created)
    --dry-run       Do everything except creating the PR
    --draft         Open the PR as a draft
-m, --model string  Use a different model (e.g. qwen2.5-coder:7b)
    --hint string   Give the model extra context about the change
-v, --verbose       Show token counts and tier selection
```

### Per-repo context

PRs often need context that isn't in the diff — coding conventions, links to tickets, what reviewers tend to care about. `bcommit context` opens a per-repo file in your `$EDITOR` that gets fed to the model as high-priority guidance whenever you generate a PR for that repo:

```bash
bcommit context          # edit this repo's context
bcommit context --path   # print the file path instead of opening it
```

The context is keyed by the repo's remote URL and stored centrally under `~/.config/bcommit/context/` — it's never committed to the repo. Lines starting with `#` are treated as comments and stripped before the model sees them.

## Configuration

bcommit stores its config at `~/.config/bcommit/config.json` (or `$XDG_CONFIG_HOME/bcommit/config.json`).

View current settings:

```bash
bcommit config
```

Change a setting:

```bash
bcommit config set model qwen2.5-coder:7b
bcommit config set auto_commit true
bcommit config set default_base develop
bcommit config set pr_reviewers alice,bob
```

| Key             | Default            | Description                                            |
|-----------------|--------------------|--------------------------------------------------------|
| `model`         | `qwen2.5-coder:3b` | Ollama model to use                                    |
| `auto_commit`   | `false`            | Skip the interactive prompt and commit                 |
| `branch_prefix` | _(none)_           | Prefix for branch names generated by `bcommit -b`      |
| `default_base`  | _(auto-detected)_  | Base branch `bcommit pr` targets                       |
| `pr_reviewers`  | _(none)_           | Comma-separated reviewers passed to `gh --reviewer`    |

## Default model

bcommit ships with `qwen2.5-coder:3b` as the default — it's a ~1.9GB code-specialized model that runs well on most machines. You can swap it out for any model available through Ollama.

## License

MIT
