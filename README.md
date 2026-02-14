# bcommit

A command-line tool that generates git commit messages using a local LLM. It reads your staged diff, sends it to a model running on [Ollama](https://ollama.com), and gives you a conventional commit message — no API keys, no cloud, everything stays on your machine.

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
bcommit config set copy_clipboard false
```

| Key              | Default             | Description                              |
|------------------|---------------------|------------------------------------------|
| `model`          | `qwen2.5-coder:3b`  | Ollama model to use                      |
| `auto_commit`    | `false`              | Skip the interactive prompt and commit   |
| `copy_clipboard` | `true`               | Copy the generated message to clipboard  |

## Default model

bcommit ships with `qwen2.5-coder:3b` as the default — it's a ~1.9GB code-specialized model that runs well on most machines. You can swap it out for any model available through Ollama.

## License

MIT
