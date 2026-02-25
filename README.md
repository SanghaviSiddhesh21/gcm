# gcm — Git Category Manager

> Organize your local git branches into categories without renaming them.

Developers accumulate dozens of local branches with no good way to organize them. Renaming branches pollutes GitHub. `gcm` solves this by storing branch-to-category metadata locally in `.git/gcm.json` — completely invisible to git and GitHub.

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap SanghaviSiddhesh21/homebrew-tap
brew install gcm
```

### Download Binary

Download the latest binary for your platform from [GitHub Releases](https://github.com/SanghaviSiddhesh21/gcm/releases).

| Platform | File |
|---|---|
| macOS (Apple Silicon) | `gcm_*_darwin_arm64.tar.gz` |
| macOS (Intel) | `gcm_*_darwin_amd64.tar.gz` |
| Linux (x86_64) | `gcm_*_linux_amd64.tar.gz` |
| Linux (ARM64) | `gcm_*_linux_arm64.tar.gz` |
| Windows (x86_64) | `gcm_*_windows_amd64.zip` |

### Build from Source

Requires Go 1.24+.

```bash
git clone https://github.com/SanghaviSiddhesh21/gcm.git
cd gcm
go build -o gcm .
```

---

## Quick Start

```bash
# 1. Initialize gcm in your repository
gcm init

# 2. Create categories
gcm create feature
gcm create hotfix
gcm create archived

# 3. Assign branches to categories
gcm assign my-new-feature feature
gcm assign critical-fix hotfix
gcm assign old-experiment archived

# 4. View all branches organized by category
gcm view

# 5. Commit with an AI-generated message
gcm commit -g
```

**Example output of `gcm view`:**

```
- feature (2 branches)
  ├── my-new-feature
  └── another-feature ●

- hotfix (1 branch)
  └── critical-fix

- Uncategorized (1 branch)
  └── main
```

The `●` marker indicates your currently checked out branch.

---

## Commands

| Command | Description |
|---|---|
| `gcm init` | Initialize gcm in the current repository |
| `gcm create <category>` | Create a new category |
| `gcm assign <branch> <category>` | Assign a branch to a category |
| `gcm view [category]` | View all branches organized by category, or filter to one |
| `gcm categories` | List all category names |
| `gcm delete <category>` | Delete a category — branches move to Uncategorized |
| `gcm commit` | Create a commit — passthrough to `git commit` |
| `gcm commit -g` | Create a commit with an AI-generated message |
| `gcm config` | Get and set configuration — passthrough to `git config` |
| `gcm config api-key <value>` | Save your Groq API key |
| `gcm config api-key` | Print your saved Groq API key |
| `gcm config --unset api-key` | Remove your saved Groq API key |

### `gcm init`

Run once per repository. Creates `.git/gcm.json` with a default `Uncategorized` category.

```bash
gcm init
```

### `gcm create <category>`

Category names must be alphanumeric and may contain `-`. Max 64 characters.

```bash
gcm create feature
gcm create hotfix
gcm create team-alice
```

### `gcm assign <branch> <category>`

Assigns a branch to a category. If the branch was already in another category, it is reassigned.

```bash
gcm assign my-feature feature
gcm assign old-branch archived
```

### `gcm view [category]`

Shows a tree of all categories with their branches. Optionally filter to one category.

```bash
gcm view           # all categories
gcm view feature   # only the feature category
```

### `gcm categories`

Lists all category names.

```bash
gcm categories
```

### `gcm delete <category>`

Deletes a category. Branches in that category are moved back to `Uncategorized`. The `Uncategorized` category itself cannot be deleted.

```bash
gcm delete archived
```

### `gcm commit`

Passthrough to `git commit`. All flags are forwarded directly.

```bash
gcm commit -m "my message"
gcm commit --amend
```

### `gcm commit -g`

Generates a commit message from your staged diff using AI and opens an interactive TUI to review it.

```
⠙ Generating commit message...

> Staged files (3)   ▼
  Unstaged files (1) ▼

Generated message:
feat(cmd): add commit and config subcommands

Accept? (y/e/r/q):
```

| Key | Action |
|---|---|
| `y` | Accept and commit |
| `e` | Edit in `$EDITOR` |
| `r` | Regenerate |
| `q` | Quit without committing |

Works out of the box using a shared API key. For higher rate limits, set your own Groq API key — see [AI Setup](#ai-setup) below.

### `gcm config`

Passthrough to `git config`. All flags are forwarded directly.

```bash
gcm config user.name "John"
gcm config --global user.email "john@example.com"
gcm config --list
```

Intercepts `api-key` to manage the GCM-specific config stored in `~/.gcm/config.json`.

```bash
gcm config api-key <your-key>   # save key
gcm config api-key              # print saved key
gcm config --unset api-key      # remove key
```

---

## AI Setup

`gcm commit -g` works out of the box with a shared API key. If you hit rate limits, set your own Groq API key for unlimited use.

**Get a free Groq API key:**

1. Sign up at [console.groq.com](https://console.groq.com)
2. Go to **API Keys** → **Create API Key**
3. Copy the key and run:

```bash
gcm config api-key <your-groq-api-key>
```

Your key is stored in `~/.gcm/config.json` and never tracked by git.

---

## How It Works

gcm stores all data in `.git/gcm.json`:

```json
{
  "version": "1.0",
  "categories": [
    { "name": "Uncategorized", "immutable": true },
    { "name": "feature", "immutable": false }
  ],
  "assignments": {
    "my-feature-branch": "feature"
  }
}
```

- Branches **not** in `assignments` are implicitly `Uncategorized`
- The file lives inside `.git/` so git never tracks it
- GitHub and your teammates never see your categories

---

## License

MIT
