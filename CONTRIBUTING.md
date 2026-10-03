# Contributing

Thanks for your interest in tg-emoji-hub. Bug reports, documentation fixes and pull requests are all welcome.

- Questions and discussion: [Telegram group](https://t.me/devqun)
- Bugs and feature requests: GitHub Issues
- Security problems: do not open a public issue; follow [SECURITY.md](SECURITY.md)

## Development environment

You need Go 1.26+, Node.js 22, Python 3.11+ and a MySQL 8 instance for running the full stack locally. The [Local development](README.md#local-development) section of the README explains how to run the server, the web UI and the crawler side by side. [docs/architecture.md](docs/architecture.md) describes how the parts fit together.

None of the automated tests need MySQL or Telegram credentials.

## Running the checks

CI (`.github/workflows/ci.yml`) runs the following on every push and pull request. Please run the ones for the parts you changed before opening a pull request.

**Go server** (repository root)

```bash
go build ./...
go vet ./...
go test ./... -count=1
```

The repository tests run against in-memory SQLite. `make test` runs the same test command.

**Web UI** (`web/`)

```bash
npm ci
npx tsc -b --force     # type check; npm run lint runs the same command
npm run build
```

Use `tsc -b`, not `tsc --noEmit`. `web/tsconfig.json` only contains project references, so `tsc --noEmit` checks no files at all and always passes.

**Crawler** (`crawler/`)

```bash
pip install -r requirements.txt
python -m pytest -q
```

`make test-py` runs the same tests from the repository root. The tests use a fake Telegram client and in-memory SQLite. Lint settings for [Ruff](https://docs.astral.sh/ruff/) are in `crawler/pyproject.toml` (`ruff check .`).

## Pull requests

- Keep each pull request focused on one change, and describe what it changes and why.
- Add or update tests when you change behavior.
- Update the documentation in the same pull request when you change the HTTP API ([docs/api.md](docs/api.md)), the MCP tools ([docs/mcp.md](docs/mcp.md)), configuration, or deployment steps.
- The HTTP handlers and the MCP tools return the same emoji object. If you change one, change the other.
- Database changes go in a new numbered pair `migrations/NNN_description.up.sql` and `.down.sql`. Update both the Go entity in `internal/domain/entity/` and the SQLAlchemy model in `crawler/lib/db.py`, because the server and the crawler write the same tables.
- The list of 11 categories exists in three places: `internal/domain/entity/emoji_sticker.go`, `crawler/backfill_keywords_ai.py` and `web/src/api/types.ts`. Keep them identical.
- Commit messages follow the [Conventional Commits](https://www.conventionalcommits.org/) style used in the history, for example `feat(crawler): ...` or `fix(frontend): ...`.
- Never commit credentials or local data: `.env` files, Telethon `.session` files, `configs/config.yaml`, database dumps, or downloaded media. `.gitignore` excludes the usual paths, but check `git status` before you commit.

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).
