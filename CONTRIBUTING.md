# Contributing

sshbox does one thing: spin up a throwaway container per SSH connection and tear it down when the session ends. It's not going to gain a plugin system, an orchestration layer, remote scheduling, or turn into a general infra ecosystem. If something belongs in Kubernetes, Nomad, or a proper orchestrator, it doesn't belong here. Keep it unix-y -- one small binary that does its one job.

## Before opening a PR

- Run `gofmt -l .` and `go vet ./...`. Both should come back clean.
- Keep changes scoped to the thing you're fixing or adding. A bug fix doesn't need a refactor riding along with it.
- No new dependencies unless the standard library genuinely can't do it.

## Style

- Don't comment what the code already says. Comment the non-obvious: a workaround, a constraint that isn't visible from the code itself, why something is the way it is.
- Don't add config options, flags, or abstractions for hypothetical future use. If nothing in the codebase needs it yet, leave it out.
- Don't add error handling or validation for cases that can't happen. Trust the Go standard library and Docker's own error surface; only guard real boundaries (the SSH session, user input, the docker binary being missing).
- Prefer a few similar lines over a shared helper that only has one caller.

## Good first issues

Check issues labeled `good-first-issue`.
