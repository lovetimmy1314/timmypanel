// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

// 统一的请求封装。会话走 HttpOnly Cookie，前端不碰 token，
// 但所有写操作必须带 X-Requested-With —— 后端用它做 CSRF 兜底。

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// 401 时通知外部（路由层）跳登录页，避免每个调用点各写一遍。
let onUnauthorized: (() => void) | null = null
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn
}

type Options = {
  method?: string
  body?: unknown
  form?: FormData
  raw?: boolean
  // 预期会 401 的探测（启动时问 /auth/me、登录、改密）不要踢回登录页。
  silent401?: boolean
}

type CallOpts = Pick<Options, 'silent401'>

export async function request<T = any>(path: string, opts: Options = {}): Promise<T> {
  const headers: Record<string, string> = { 'X-Requested-With': 'Timmypanel' }
  let body: BodyInit | undefined

  if (opts.form) {
    body = opts.form // 让浏览器自己带 multipart 边界，不要手动设 Content-Type
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  const res = await fetch(`/api/v1${path}`, {
    method: opts.method ?? (body ? 'POST' : 'GET'),
    headers,
    body,
    credentials: 'same-origin',
  })

  if (res.status === 401) {
    let msg = '未登录或登录已过期'
    try {
      const data = await res.json()
      if (data?.error) msg = data.error
    } catch {
      /* 响应不是 JSON，用默认文案 */
    }
    // 登录失败、改密原密码不对也是 401。只有会话中间件那句才踢回登录页；
    // silent401 留给「我知道会 401」的探测，避免和业务 401 抢消息。
    if (!opts.silent401 && msg === '未登录或登录已过期') onUnauthorized?.()
    throw new ApiError(401, msg)
  }
  if (!res.ok) {
    let msg = `请求失败 (${res.status})`
    try {
      const data = await res.json()
      if (data?.error) msg = data.error
    } catch {
      /* 响应不是 JSON，用默认文案 */
    }
    throw new ApiError(res.status, msg)
  }
  if (opts.raw) return res as unknown as T
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  get: <T = any>(p: string, opts?: CallOpts) => request<T>(p, opts),
  post: <T = any>(p: string, body?: unknown, opts?: CallOpts) =>
    request<T>(p, { ...opts, method: 'POST', body }),
  put: <T = any>(p: string, body?: unknown, opts?: CallOpts) =>
    request<T>(p, { ...opts, method: 'PUT', body }),
  patch: <T = any>(p: string, body?: unknown, opts?: CallOpts) =>
    request<T>(p, { ...opts, method: 'PATCH', body }),
  del: <T = any>(p: string, opts?: CallOpts) => request<T>(p, { ...opts, method: 'DELETE' }),
  upload: <T = any>(p: string, form: FormData, opts?: CallOpts) =>
    request<T>(p, { ...opts, method: 'POST', form }),
}
