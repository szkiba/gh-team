# Tasks

## 1. Project scaffolding
- [ ] 1.1 Create Go module and `gh` extension manifest (`gh-team` binary)
- [ ] 1.2 Set up Cobra root command with global flags (`--ownership`, `--direct-only`, `--include-archived`)
- [ ] 1.3 Wire `--direct-only` + `--ownership=codeowners` validation error at the root

## 2. Ownership resolver
- [ ] 2.1 Define `Resolver` interface returning `[]Repo` for a team slug
- [ ] 2.1a Implement team-existence preflight (`GET /orgs/<org>/teams/<team-slug>`) shared by both strategies; cache result for the invocation
- [ ] 2.2 Implement `permission` strategy
  - [ ] 2.2.1 Fetch repos for the team with permission filter (`Admin` | `Maintain`)
  - [ ] 2.2.2 Recursively walk sub-teams unless `--direct-only`
  - [ ] 2.2.3 Deduplicate results across team + sub-teams
- [ ] 2.3 Implement `codeowners` strategy
  - [ ] 2.3.1 Run code search `org:<ORG> path:CODEOWNERS "@<ORG>/<TEAM>"` to collect candidate repositories (broad query catches multi-owner wildcard lines and whitespace variations)
  - [ ] 2.3.2 For each candidate, fetch the effective CODEOWNERS file from the first existing of `.github/CODEOWNERS`, `CODEOWNERS`, `docs/CODEOWNERS` on the default branch; reject candidates with no file at any of those paths
  - [ ] 2.3.3 Parse fetched files: strip `#…EOL` comments per line; ignore blank lines; tokenize remaining lines on whitespace where first token = pattern and rest = owners
  - [ ] 2.3.4 Apply ownership rule: the **last** parsed line whose first token is exactly `*` must list `@<ORG>/<TEAM>` as one of its owners (case-insensitive on the slug); earlier wildcard lines are superseded
- [ ] 2.4 Apply `--include-archived` filter (default: drop archived)
- [ ] 2.5 Sort results alphabetically by repo name

## 3. `team repo` subcommands
- [ ] 3.1 `team repo list <org/team-slug>` — print names, one per line
- [ ] 3.2 `team repo clone <org/team-slug>` — invoke `gh repo clone` for each
  - [ ] 3.2.1 Skip already-cloned directories with a non-fatal warning
  - [ ] 3.2.2 Aggregate clone failures, exit non-zero if any failed

## 4. Validation & errors
- [ ] 4.1 Reject malformed `<org/team-slug>` arguments with usage hint
- [ ] 4.1a Reject missing team / missing org with clear stderr error (preflight 404)
- [ ] 4.2 Reject `--direct-only --ownership=codeowners`
- [ ] 4.3 Surface GitHub API auth errors with actionable guidance (`gh auth login`)
- [ ] 4.4 Surface GitHub API rate-limit errors with reset time

## 5. Testing
- [ ] 5.1 Unit tests for both ownership strategies
- [ ] 5.2 CODEOWNERS parser tests (comment stripping, blank lines, precedence: last `*` wins, multi-owner lines, path-scoped lines ignored, file resolution `.github/CODEOWNERS` > `CODEOWNERS` > `docs/CODEOWNERS`)
- [ ] 5.3 Integration tests with recorded GitHub API fixtures

## 6. Docs
- [ ] 6.1 README with install + usage examples
- [ ] 6.2 `--help` output for every subcommand
