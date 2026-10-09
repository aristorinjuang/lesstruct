# modules/auth

Official Lesstruct module: authentication — credential login with JWT
sessions, self-service registration, email verification, password reset,
and first-login bootstrap, all behind async-tolerant admin handlers.

## Layout (module template)

```
module.go            # kernel.Module (single registry-facing file)
api/                 # browser handlers (agent UPPER_SNAKE envelope) + seams
domain/              # Auth/Login/Registration/Verification/PasswordReset services (4.0c)
admin/               # admin-shell manifest (empty: auth is chrome)
testutil/            # kernel fixture for module tests
```

## Handlers

The `api.AuthHandler` serves login, registration, verification, and
password reset; `api.FirstLoginHandler` serves setup status and completion.
Both write through `kernel/httpx` (byte-identical to the pre-module
`internal/api/response` writers by contract). Login mints JWTs through the
`TokenIssuer` seam (verification stays core-side in middleware); account
emails flow through the `Mailer` seam; admin notifications through the
`NotificationRecorder` seam (core adapter persists them with identical
escaped text). Core mounts the handlers explicitly through structural
`AuthSurface`/`FirstLoginSurface` interfaces in `routes.go`, so routes stay
module-agnostic and paths never drift.
