// Thin JSON client for the Fond API.

export class ApiError extends Error {
  constructor(public status: number, message: string, public data: any = null) {
    super(message);
  }
}

export let onUnauthorized: () => void = () => {};
export function setUnauthorizedHandler(fn: () => void) {
  onUnauthorized = fn;
}

export async function api<T = any>(method: string, path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, credentials: 'same-origin', headers: {} };
  if (body instanceof FormData) init.body = body;
  else if (body !== undefined) {
    (init.headers as Record<string, string>)['Content-Type'] = 'application/json';
    init.body = JSON.stringify(body);
  }
  let res: Response;
  try {
    res = await fetch('/api' + path, init);
  } catch {
    throw new ApiError(0, 'Fond can’t reach the server. Check your connection.');
  }
  let data: any = null;
  const text = await res.text();
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = null;
  }
  if (!res.ok) {
    if (res.status === 401 && !path.startsWith('/auth/') && !path.startsWith('/public/')) onUnauthorized();
    throw new ApiError(res.status, data?.error || `Request failed (${res.status})`, data);
  }
  return data as T;
}

export const get = <T = any>(p: string) => api<T>('GET', p);
export const post = <T = any>(p: string, b?: unknown) => api<T>('POST', p, b ?? {});
export const put = <T = any>(p: string, b?: unknown) => api<T>('PUT', p, b ?? {});
export const del = <T = any>(p: string) => api<T>('DELETE', p);

export const img = (key: string, thumb = false) => `/img/${key}${thumb ? '_t' : ''}.jpg`;
