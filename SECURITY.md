# Security Policy

## Implemented safeguards

The current application includes bcrypt password hashing, HTTP-only `SameSite=Lax` session cookies, rate limiting for login and public-share unlock requests, and encryption for stored S3-compatible credentials. The login and share-unlock rate limiter is in memory and resets when the server restarts.

These controls do not replace host security, HTTPS, backups, or access control for the storage volume. CubyCloud does not claim end-to-end encryption for uploaded file contents.

## Reporting a vulnerability

This repository does not currently publish a dedicated security contact or a response-time commitment. **Do not post exploit details, credentials, or sensitive reproduction data in a public issue.**

If private vulnerability reporting is enabled for the repository, use that channel. Otherwise, open a non-sensitive issue asking the maintainers to provide a private disclosure channel; do not include technical details until a private route has been agreed.
