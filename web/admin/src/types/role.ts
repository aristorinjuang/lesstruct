/**
 * Role catalog types for the admin panel. Roles are config-driven
 * ([[role]] in config.toml) and surfaced by GET /api/v1/roles: the full
 * assignable catalog plus the caller's own capabilities (`me`).
 */

export interface Role {
  name: string
  postTypes?: string[]
  allTypes: boolean
  publish: boolean
  media: boolean
  comments: boolean
  isAdmin: boolean
}

export interface MeCapabilities {
  role: string
  postTypes: string[]
  publish: boolean
  media: boolean
  comments: boolean
  isAdmin: boolean
}

export interface RolesResponse {
  data: {
    roles: Role[]
    me: MeCapabilities
  }
  error: null
  meta?: { timestamp: string }
}