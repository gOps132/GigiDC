# Gigi Documentation Commit Message Template

Use this when a commit changes docs, planning notes, architecture records, setup guides, or other project documentation.

## Short Commit Title

```text
docs: <clear summary>
```

Examples:

```text
docs: document EC2 deploy workflow
docs: clarify slash command permission model
docs: add architecture notes for V1 bot flow
```

## Extended Commit Message Template

```text
docs: <clear summary>

What changed:
- <doc/page/file updated>
- <main clarification or addition>
- <secondary clarification or addition>

Why:
- <reason this documentation change was needed>

Validation:
- Reviewed for sensitive data leakage
- Checked commands, paths, and examples against current repo state
- <other verification performed, if any>

External resources:
- None
```

## When External Material Is Used

Replace the `External resources` section with explicit attribution:

```text
External resources:
- <resource/provider name> — used for <purpose> — <link if practical>
```

## Lightweight Variant

Use this shorter version for small doc-only commits:

```text
docs: <clear summary>

- update: <what changed>
- reason: <why>
- validation: reviewed for sensitive data leakage
- external resources: none
```
