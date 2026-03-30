# Human-Agent Workflow

This doc defines the default collaboration model for humans and Codex when working on `gigi`, especially when multiple Codex terminals are active at once.

## Goals

- avoid file and branch conflicts between parallel Codex sessions
- make ownership of work explicit
- make it easy to answer "what are you working on right now?"
- keep large feature and architecture work reviewable

## Default Rule

Use separate Git worktrees for parallel Codex sessions.

- one Codex terminal should own one branch
- one branch should live in one worktree
- one worktree should have one clearly defined scope

Do not run multiple Codex terminals against the same working tree when both may edit files.

## Recommended Worktree Setup

Start from an up-to-date `main` checkout, then create separate worktrees for each active slice:

```bash
git fetch origin
git switch main
git pull --ff-only

git worktree add ../gigi-feature-x -b feat/feature-x main
git worktree add ../gigi-tests-feature-x -b test/feature-x main
git worktree add ../gigi-arch-spike -b spike/arch-change main
```

Suggested usage:

- use a feature worktree for the main implementation
- use a test or docs worktree for non-overlapping follow-up work
- use a spike worktree for architecture exploration that should not destabilize implementation in progress

## Scope Ownership

Every active Codex terminal should have an explicit owned scope before it starts editing.

A scope should include:

- branch name
- worktree path
- objective
- owned files or owned layer
- expected validation

Example:

```text
Scope: add DM relay confirmation buttons
Branch: feat/dm-relay-confirmation
Worktree: ../gigi-dm-relay-confirmation
Owns: src/services/ActionConfirmationService.ts, src/discord/client.ts, related tests
Validation: npm run typecheck, npm run build, targeted tests
```

## How To Split Work Safely

Split work by ownership boundaries, not by convenience.

Good parallel splits:

- Discord interaction surface vs service implementation
- service implementation vs tests
- ports/interfaces vs adapters, once the interface owner has fixed the contract
- docs/diagrams vs code, after the implementation direction is stable

Bad parallel splits:

- two terminals editing the same service file
- two terminals changing the same Supabase migration
- two terminals independently redefining shared types or service contracts
- two terminals making overlapping "small cleanup" edits across the same area

## Feature Work

For a normal feature:

1. choose one coordinating terminal
2. define the feature slices and file ownership
3. assign one non-overlapping slice per worktree
4. merge or cherry-pick worker branches into a single integration branch
5. let the coordinating terminal handle final conflict resolution and final verification

Keep the coordinator responsible for:

- shared interfaces
- rebases and integration
- final validation
- PR assembly

## Large Architectural Changes

Do not start architecture work with parallel coding by default.

For architecture-heavy changes:

1. use one terminal to analyze the current system first
2. confirm the target seams, responsibilities, and migration plan
3. update architecture docs and diagrams if the mental model changes
4. only then split implementation across multiple worktrees

In this repo, architecture work should follow the project-visualization workflow in [docs/project-visualization-workflow.md](/Users/giancedrick/dev/projects/gigi/docs/project-visualization-workflow.md) and update [docs/architecture-v1.md](/Users/giancedrick/dev/projects/gigi/docs/architecture-v1.md) when the system model changes.

For large changes, prefer ownership by layer:

- runtime and Discord surface
- application services
- ports and shared interfaces
- adapters and persistence
- tests and docs

Only one terminal should define a new shared contract. Other terminals should implement against that contract after it is stable.

## Single-Owner Areas

These areas should usually have exactly one active owner at a time:

- `package.json`
- shared environment/config files
- shared service interfaces and cross-cutting types
- Supabase migration creation and numbering
- CI/CD workflow files
- deploy scripts
- architecture docs and diagrams during the same architecture change

## Status Reporting

When a human asks a Codex terminal what scope it is working on, the answer should be concrete and operational.

Preferred status format:

```text
Current scope: <objective>
Branch: <branch-name>
Worktree: <path>
Owns: <files or subsystem>
Doing now: <current task>
Blocked by: <nothing | specific dependency>
Next validation: <command or test>
```

Example:

```text
Current scope: DM assignment admin flow
Branch: feat/dm-assignment-admin
Worktree: ../gigi-dm-assignment-admin
Owns: src/services/GuildAdminActionService.ts, src/services/AgentToolService.ts, related tests
Doing now: wiring DM assignment create/list/publish calls into shared guild capability checks
Blocked by: nothing
Next validation: npm run typecheck
```

This format is preferred over vague answers like "working on the DM stuff."

## Integration Rules

Before integration:

- rebase each worker branch onto the current target branch
- verify ownership boundaries are still accurate
- confirm no other active terminal is editing the same files

During integration:

- use one integration branch
- let one terminal resolve conflicts
- rerun validation after conflicts are resolved

## Validation Expectations

Each worker should run the smallest relevant checks for its scope.

The integration owner should run the final repo-standard validation appropriate to the change, including:

- `npm run typecheck`
- `npm run build`
- targeted tests for the touched area
- `terraform fmt -check` and `terraform validate` when Terraform changed

## Practical Limits

Multiple Codex terminals help when the work is truly separable.

Usually:

- one terminal is enough for a small change
- two terminals is reasonable for a medium feature
- three terminals is the practical upper bound before coordination cost rises sharply

If two scopes are touching the same files, the split is probably wrong and should be collapsed back to one owner.
