# Project Agents Conventions

## Operational Memory

* `api/` -> Contains Go serverless functions deployed on Vercel.
  * `api/feeds.go` -> Main handler for generating Nix OS channel RSS feeds. Includes functions for downloading channel history and formatting it as feeds.
* `vercel.json` -> Configuration for Vercel deployment, including URL rewrites.
* `index.html` -> Static frontend served at the root URL.
* `scripts/workspaced` -> Custom linting and formatting script wrapping `go vet` and `gofmt`. Configured to warn (exit 0) on formatting errors.

## Tooling Constraints

* **Task Runner:** `mise` is the required tool for managing tasks and tool versioning. It must be installed and used for all automated checks.
* **Tool Pinning:** `mise` tools must be pinned to specific versions (e.g., Go 1.24.3). Do not use "latest" or "lts".
* **Linting:** Use `mise run lint` for verifying code before commits.

## Error Handling Guidelines

* **Centralized Reporting:** All unexpected errors must be funneled through a centralized reporting mechanism if one exists.
* **No Silent Failures:** Never leave an empty catch block or swallow errors silently. All unrecoverable errors must be reported or properly propagated.
* **Do Not Fix Code:** Do not modify executable logic to fix issues unless explicitly instructed. Refactoring is strictly forbidden under the documentation scope.
