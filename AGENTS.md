# AGENTS.md

## impl default settings

When running `$impl` in this repository, use the following defaults unless explicitly overridden by the user:

- Review method: self review
- Check method: auto detect from project files (`package.json`, `Makefile`, `pyproject.toml`, `Cargo.toml`, etc.)

## note

If check commands fail due to intentional WIP or red-phase tests, allow commit only with an explicit note in the commit message.
