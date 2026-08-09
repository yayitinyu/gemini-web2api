# Security policy

Please do not disclose credential-handling or authentication vulnerabilities in a public issue.
Use GitHub's **Report a vulnerability** flow for this repository so the report can be discussed privately.

The `main` branch is the only supported version before the first tagged release.

Operational guidance:

- Put the admin panel behind HTTPS and set `COOKIE_SECURE=true`.
- Keep `/data`, `.env`, Google cookies, API keys, and encryption keys out of backups that are not encrypted.
- Only enable `TRUST_PROXY_HEADERS` when every request comes through a trusted reverse proxy.
- Rotate the OpenAI-compatible API key after accidental disclosure. Existing keys stop working immediately.
- Restarting the gateway revokes all existing admin sessions, including sessions created before an admin-password change.
- This project stores request metadata, but never stores prompts or generated response bodies.
