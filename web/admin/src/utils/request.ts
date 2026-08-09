export class ApiError extends Error {
  statusCode: number
  code?: string

  constructor(message: string, statusCode: number, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.statusCode = statusCode
    this.code = code
  }
}

const BASE_URL = import.meta.env.VITE_API_BASE_URL || ''
const TIMEOUT = 10000

interface RequestOptions {
  headers?: HeadersInit
  params?: Record<string, string | number>
  timeout?: number
}

function extractFilename(disposition: string | null, fallback: string): string {
  if (!disposition) {
    return fallback
  }
  const match = /filename="([^"]+)"/.exec(disposition) || /filename=([^;]+)/.exec(disposition)
  const filename = match?.[1]
  if (!filename) {
    return fallback
  }
  return filename.trim().replace(/^["']|["']$/g, '')
}

/**
 * Download a binary response (e.g. a tar.gz archive) as a file attachment.
 * No timeout by default — large exports can take minutes, mirroring the CLI
 * and Bruno collections (timeout: 0). Returns the server-provided filename.
 */
async function download(
  path: string,
  fallbackFilename: string,
  options?: { timeout?: number }
): Promise<string> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {}
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }

  const url = `${BASE_URL}${path}`
  const controller = new AbortController()
  const timeoutMs = options?.timeout ?? 0
  const timeoutId = timeoutMs > 0 ? setTimeout(() => controller.abort(), timeoutMs) : null

  try {
    const response = await fetch(url, {
      method: 'GET',
      headers,
      signal: controller.signal,
    })

    if (!response.ok) {
      let code: string | undefined
      let message = response.statusText
      try {
        const body = await response.json()
        if (body?.error?.code) {
          code = body.error.code
        }
        if (body?.error?.message) {
          message = body.error.message
        }
      } catch {
        // Response body not parseable as JSON, use status text
      }
      throw new ApiError(message, response.status, code)
    }

    const blob = await response.blob()
    const filename = extractFilename(response.headers.get('Content-Disposition'), fallbackFilename)
    const objectUrl = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = objectUrl
    anchor.download = filename
    document.body.appendChild(anchor)
    anchor.click()
    document.body.removeChild(anchor)
    setTimeout(() => URL.revokeObjectURL(objectUrl), 0)
    return filename
  } catch (err) {
    if (timeoutId !== null) {
      clearTimeout(timeoutId)
    }
    throw err
  }
}

async function request<T>(
  method: string,
  path: string,
  data?: Record<string, unknown> | FormData,
  options?: RequestOptions
): Promise<{ data: T }> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {
    ...(options?.headers as Record<string, string>),
  }

  // Only set Content-Type for JSON requests (not FormData)
  if (!(data instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }

  if (token) {
    headers.Authorization = `Bearer ${token}`
  }

  // Build URL with query params
  let url = `${BASE_URL}${path}`
  if (options?.params) {
    const searchParams = new URLSearchParams()
    for (const [key, value] of Object.entries(options.params)) {
      searchParams.append(key, String(value))
    }
    if (searchParams.toString()) {
      url += `?${searchParams.toString()}`
    }
  }

  const controller = new AbortController()
  const timeoutMs = options?.timeout ?? TIMEOUT
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs)

  try {
    const response = await fetch(url, {
      method,
      headers,
      body: data instanceof FormData ? data : data ? JSON.stringify(data) : undefined,
      signal: controller.signal
    })

    clearTimeout(timeoutId)

    if (!response.ok) {
      // Try to parse structured error response from backend
      let code: string | undefined
      let message = response.statusText

      try {
        const body = await response.json()
        if (body?.error?.code) {
          code = body.error.code
        }
        if (body?.error?.message) {
          message = body.error.message
        }
      } catch {
        // Response body not parseable as JSON, use status text
      }

      if (response.status === 403 && code === 'CSRF_VALIDATION_FAILED') {
        throw new ApiError(
          'Your session could not be verified. Please refresh the page and try again.',
          response.status,
          code,
        )
      }

      throw new ApiError(message, response.status, code)
    }

    const json = response.status === 204 ? null : await response.json()
    return { data: json }
  } catch (err) {
    clearTimeout(timeoutId)
    throw err
  }
}

export default {
  get<T>(path: string, options?: RequestOptions) {
    return request<T>('GET', path, undefined, options)
  },

  post<T>(path: string, data: Record<string, unknown> | FormData, options?: RequestOptions) {
    return request<T>('POST', path, data, options)
  },

  postWithTimeout<T>(path: string, data: Record<string, unknown> | FormData, timeoutMs: number, options?: RequestOptions) {
    return request<T>('POST', path, data, { ...options, timeout: timeoutMs })
  },

  put<T>(path: string, data: Record<string, unknown> | FormData, options?: RequestOptions) {
    return request<T>('PUT', path, data, options)
  },

  patch<T>(path: string, data: Record<string, unknown>, options?: RequestOptions) {
    return request<T>('PATCH', path, data, options)
  },

  delete<T>(path: string) {
    return request<T>('DELETE', path)
  },

  download(path: string, fallbackFilename: string, options?: { timeout?: number }) {
    return download(path, fallbackFilename, options)
  }
}
