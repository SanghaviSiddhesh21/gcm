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

Requires Go 1.21+.

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

# 5. View a specific category
gcm view feature
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

### `gcm init`

Run once per repository. Creates `.git/gcm.json` with a default `Uncategorized` category.

```bash
gcm init
```

### `gcm create <category>`

Category names must be alphanumeric and may contain `-` or `_`. Max 64 characters.

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
