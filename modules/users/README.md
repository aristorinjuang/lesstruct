# modules/users

Official Lesstruct module: user management — admin user lifecycle
(pending approval, suspension, soft delete with content restore, admin
creation), self-service profile (email, password, name, custom fields, data
export, account deletion), and profile pictures, all behind async-tolerant
admin handlers.

## Layout (module template)

```
module.go            # kernel.Module (single registry-facing file)
api/                 # browser handlers (agent UPPER_SNAKE envelope) + seams
domain/              # UserManagement/Profile/AccountDeletion/AdminCreation services (4.0b)
domain/profilepicture/ # profile picture service, processor, kernel-storage seam (4.0e)
admin/               # admin-shell manifest (empty: Users nav is static SPA chrome)
testutil/            # kernel fixture for module tests
```

## Handlers

The `api.UserManagementHandler` serves the admin user lifecycle;
`api.ProfileHandler` serves self-service profile; `api.ProfilePictureHandler`
serves picture upload/delete for self and admin. All three write through
`kernel/httpx` (byte-identical to the pre-module `internal/api/response`
writers by contract). Account emails flow through the `Mailer` seam (the six
user-management sends); soft-delete reads flow through the `SoftDeleteStore`
seam (core adapter converts database null types at the boundary); handler
user lookups flow through the narrow `UserStore` seam (the core user
repository satisfies it structurally). The pre-module `jwtManager` fields
were dead weight (stored, never called) and were dropped with zero behavior
change.
