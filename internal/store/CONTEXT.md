# internal/store

JSON persistence layer for the `.git/gcm.json` file. Owns the data model, validation rules, and all CRUD operations on categories and branch assignments.

## Public API

- **Store** — The top-level data structure: version string, ordered list of categories, and a branch→category assignment map.
- **Category** — A named group with an `Immutable` flag. Only "Uncategorized" is immutable.
- **NewStore** — Creates a fresh store with only the immutable "Uncategorized" category.
- **Load / Save** — Read from / write to `<gitDir>/gcm.json`. Save is atomic (write to `.tmp`, rename).
- **ValidateCategoryName** — Enforces `^[a-zA-Z0-9-]+$`, max 64 chars, rejects the reserved name "Uncategorized".
- **Store methods** — `AddCategory`, `RemoveCategory`, `AssignBranch`, `BranchesInCategory`, `CategoryExists`, `GetCategory`, `GetAssignment`.

## Key Invariants

- "Uncategorized" always exists, is always immutable, and cannot be created or deleted by user action.
- Branches without an explicit assignment are implicitly Uncategorized. `GetAssignment` returns `UncategorizedName` for unknown branches.
- `BranchesInCategory` requires a live branch list from git — it filters assignments against actually-existing branches, so stale assignments (for deleted branches) are excluded from results.
- `RemoveCategory` deletes all assignments pointing to the removed category. Those branches become implicitly Uncategorized.
- Sentinel errors (`ErrNotInitialized`, `ErrCategoryNotFound`, etc.) are used for all error conditions. No dynamic error wrapping.

## Gotchas

- The store has no migration logic. If the schema changes, old `gcm.json` files will fail to parse.
- `ValidateCategoryName` compiles a new regex on every call. Not a performance concern at current usage levels, but worth noting.
- Category names reject underscores despite the README claiming they're allowed. This is a known doc-code mismatch.

## Testing

Table-driven tests cover `ValidateCategoryName` exhaustively (valid names, edge cases, reserved names). Other tests are straightforward unit tests covering CRUD operations, save/load round-tripping, atomic write cleanup, corrupted file handling, and the stale-assignment-exclusion behavior.
