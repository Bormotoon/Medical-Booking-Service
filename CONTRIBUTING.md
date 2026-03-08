# Contributing

Thank you for considering a contribution.

## Workflow

1. Open an issue first for large changes.
2. Keep pull requests focused.
3. Add or update tests for behavior changes.
4. Run the relevant checks before opening a PR.

## Recommended Checks

```bash
make ci
```

Or targeted commands:

```bash
make test-jr
make test-crm
make build-jr
make build-crm
```

## Pull Requests

- explain the problem and the solution;
- describe validation steps;
- mention rollout or migration risks when relevant;
- do not commit `.env`, credentials, databases, or generated binaries.

## Licensing

By contributing, you agree that your contributions are distributed under the repository license.
