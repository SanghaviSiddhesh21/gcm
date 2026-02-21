# GCM — Git Category Manager

A lightweight tool for organizing git branches by category without renaming them. Categories are stored locally in `.git/gcm.json`, completely invisible to GitHub.

## Overview

Developers with many local git branches have no lightweight way to organize them without renaming branches (which pollutes GitHub). `gcm` solves this by storing branch-to-category metadata locally.

## Features

- **Local-only storage**: Categories stored in `.git/gcm.json` (never tracked by git)
- **Non-destructive**: Branch names never change—GitHub always sees original names
- **Immutable default**: `Uncategorized` category always exists and cannot be deleted
- **Visual organization**: Tree view of all branches grouped by category
- **Easy switching**: `gcm switch` to jump between branches with category context

## Installation

### From Homebrew
```bash
brew install siddhesh/tap/gcm
```

### From GitHub Releases
Download the binary for your OS from [releases](https://github.com/siddhesh/gcm/releases).

### Build from Source
```bash
git clone https://github.com/siddhesh/gcm.git
cd gcm
go build -o gcm ./cmd/main.go
```

## Quick Start

```bash
# Initialize gcm in your repo
gcm init

# Create a category
gcm create feature
gcm create archived

# Assign a branch to a category
gcm assign my-new-feature feature
gcm assign old-experiment archived

# View all branches organized by category
gcm view

# View one category
gcm view feature

# Switch to a branch
gcm switch my-new-feature

# List all categories
gcm categories

# Delete a category (branches move to Uncategorized)
gcm delete archived
```

## Commands

| Command | Description |
|---|---|
| `gcm init` | Initialize gcm in repo + set up git user config |
| `gcm create <category>` | Create a new custom category |
| `gcm categories` | List all categories |
| `gcm assign <branch> <category>` | Assign a branch to a category |
| `gcm switch <branch>` | Switch to any branch (shows category context) |
| `gcm view` | Visual tree of ALL categories + branches |
| `gcm view <category>` | Show only branches in a specific category |
| `gcm delete <category>` | Delete category—branches move to Uncategorized |

## Storage Format

GCM stores metadata in `.git/gcm.json`:

```json
{
  "version": "1.0",
  "categories": [
    { "name": "Uncategorized", "immutable": true },
    { "name": "feature", "immutable": false },
    { "name": "archived", "immutable": false }
  ],
  "assignments": {
    "my-branch-name": "feature",
    "old-branch": "archived"
  }
}
```

## Project Structure

```
gcm/
├── main.go
├── cmd/
│   ├── root.go         # Root cobra command, version info
│   ├── init.go         # gcm init
│   ├── create.go       # gcm create
│   ├── categories.go   # gcm categories
│   ├── assign.go       # gcm assign
│   ├── switch.go       # gcm switch
│   ├── view.go         # gcm view / gcm view <category>
│   └── delete.go       # gcm delete
├── internal/
│   ├── store/
│   │   └── store.go    # Read/write .git/gcm.json
│   ├── git/
│   │   └── git.go      # List branches, current branch, switch branch, etc.
│   └── ui/
│       └── ui.go       # Tree rendering, colors, formatting
├── go.mod
├── go.sum
└── README.md
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/fatih/color` — Terminal color output

## Development

### Checkpoints

1. **Checkpoint 1**: Project setup (Go module, Cobra CLI, directory structure)
2. **Checkpoint 2**: Storage layer (gcm.json read/write, gcm init)
3. **Checkpoint 3**: Git integration (branch listing, switching)
4. **Checkpoint 4**: Core commands (create, categories, assign, delete, switch)
5. **Checkpoint 5**: Visual layer (gcm view with tree rendering)
6. **Checkpoint 6**: Distribution (GitHub Actions, Homebrew)

## License

MIT
