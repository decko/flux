const API_BASE = '/api/v1';

class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const url = `${API_BASE}${path}`;
  const options: RequestInit = {
    method,
    headers: {
      'Content-Type': 'application/json',
    },
  };

  if (body !== undefined) {
    options.body = JSON.stringify(body);
  }

  const response = await fetch(url, options);

  if (!response.ok) {
    const message = await response.text().catch(() => response.statusText);
    throw new ApiError(message, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

export async function apiGet<T>(path: string): Promise<T> {
  return request<T>('GET', path);
}

export async function apiPost<T>(
  path: string,
  body: unknown,
): Promise<T> {
  return request<T>('POST', path, body);
}

export async function apiPut<T>(
  path: string,
  body: unknown,
): Promise<T> {
  return request<T>('PUT', path, body);
}

export async function apiDelete(path: string): Promise<void> {
  return request<void>('DELETE', path);
}
