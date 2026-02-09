# Track CLI Reference

Use these commands as a compact playbook for issue operations.

## Discovery

```bash
track list
track list --status todo
track show TRK-5
```

## State Updates

```bash
track set TRK-5 --status in_progress
track set TRK-5 --priority p2
track done TRK-5
```

## Labels

```bash
track label attach TRK-5 backend blocked
track label detach TRK-5 blocked
```

## Work Chaining

```bash
track next TRK-5 "Implement first slice"
```

## Notes

- Always inspect with `track show <id>` before mutation.
- Prefer one logical tracker update per step for clearer history.
