/**
 * API Key admin types — mirror the backend DTOs in internal/api/handlers/apikey.go.
 * Backend is FROZEN (Stories 1.1 + 1.2); these shapes must match exactly.
 */

export const KEY_PREFIX = 'lesstruct_'

/** List item from GET /api/admin/api-keys (masked — never contains the secret/hash). */
export interface ApiKey {
  id: number
  name: string
  prefix: string
  createdAt: string
  lastUsedAt: string | null
  expiresAt: string | null
  revokedAt: string | null
}

/** Response from POST /api/admin/api-keys — the full key is shown ONCE. */
export interface CreateApiKeyResponse {
  key: string
  id: number
  name: string
  keyPrefix: string
  createdAt: string
}

/** Response from DELETE /api/admin/api-keys/:id. */
export interface RevokeApiKeyResponse {
  id: number
  revokedAt: string
}
