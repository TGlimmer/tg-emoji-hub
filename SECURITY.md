# Security Policy

## Reporting a vulnerability

Please do not report security problems in public issues, pull requests or the Telegram group.

Report them privately through GitHub's private vulnerability reporting: open the repository's **Security** tab and choose **Report a vulnerability**. Include the affected version or commit, steps to reproduce, and the impact you expect.

Fixes are made on the `main` branch; there are no separately maintained release branches.

## Protect your Telegram credentials

The crawler logs in to Telegram as a real user account. Treat the following as secrets, in the same way as your Telegram password:

| What | Where | Why it matters |
|------|-------|----------------|
| Telethon session file (`*.session`) | `crawler/.session/` for a local or bare-metal setup; the `tg_session` volume in Docker | It contains the authorization key of a logged-in session. Whoever has the file can act as your account without your password or a login code. |
| `crawler/.env` | Local or bare-metal setup | Contains `TG_API_ID`, `TG_API_HASH`, your phone number, the database password and any AI provider keys. |
| `deployments/.env` | Docker setup | Contains the same Telegram values, the MySQL passwords and any AI provider keys. |

- Never commit these files, attach them to issues, or include them in screenshots or shared archives. `.gitignore` and `.dockerignore` exclude them, but backups of the project directory or of Docker volumes still contain them.
- If a session file may have leaked, end that session in Telegram under **Settings → Devices** and delete the file. Then log in again to create a new one.
- Using a separate Telegram account for the crawler limits the damage if its session is ever exposed.

## Deployment notes

- The server has no authentication. Every endpoint is open, including adding packs (`POST /api/v1/packs`) and the MCP endpoint (`/mcp`). Do not expose it to the internet without a reverse proxy that adds HTTPS and access control.
- In the Docker deployment only the web port (`HTTP_PORT`) is published. MySQL and the Go server are reachable only inside the Compose network. Keep it that way in production.
- Use strong, unique database passwords, and keep AI provider API keys only in the `.env` file of the machine that runs the crawler.
