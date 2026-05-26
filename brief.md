# Feature Brief: gh team CLI Extension

Engineering teams lack a single, consistent way to see the full list of repositories they own or inherit across a GitHub organization. Finding a team's total list of repositories requires writing complex API scripts or clicking through dozens of browser tabs. This makes routine maintenance manual and slow for the team itself.

## What we are building

We are building the **`gh team` CLI extension**: a tool for the GitHub CLI (`gh`) to discover repositories based on a configurable **Team Ownership Model**.

Two ownership strategies are supported, selected via `--ownership`:

* **`permission`** (default): a repository belongs to a team if that team or any of its sub-teams has the **`Admin`** or **`Maintain`** permission on it (matching GitHub's `permission` field on team-to-repository bindings).
* **`codeowners`**: a repository belongs to a team if the team is named as the owner of the bare `*` wildcard pattern in the repository's effective `CODEOWNERS` file on the default branch (respecting CODEOWNERS' last-matching-pattern precedence).

This lets a team instantly see and manage its entire list of code assets.

## User experience

The MVP is intentionally narrow: repository discovery and cloning only.

### Repository Management (`repo`)
* **`gh team repo list <org/team-slug>`**: Outputs a clean, alphabetical list of repository names (one per line) for easy scripting.
* **`gh team repo clone <org/team-slug>`**: Clones all of these repositories into the current directory (delegating to `gh repo clone`).

### Global Flags
The behavior of any command can be changed with these flags:
* **`--ownership=permission|codeowners`**: Selects the ownership strategy. Defaults to `permission`.
* **`--direct-only`**: Only evaluates repositories assigned directly to the top-level team, skipping sub-teams completely. Only valid with `--ownership=permission`.
* **`--include-archived`**: Includes archived repositories in the list or clone queue (the default targets active repositories only).

## Out of scope for the MVP

Security and vulnerability tracking (`gh team security summary`, `gh team security alerts`) was part of the original brief but is deferred to a follow-up release so the MVP can ship focused on the repository-discovery foundation. A separate OpenSpec change will add it on top of the same ownership resolver.
