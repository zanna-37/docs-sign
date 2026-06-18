export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

// errMessage returns the server-provided message for an ApiError, falling back to a
// caller-supplied (already-translated) message for any other failure.
export function errMessage(err: unknown, fallback: string): string {
  return err instanceof ApiError ? err.message : fallback;
}

async function request(path: string, init?: RequestInit): Promise<Response> {
  return fetch("/api" + path, {
    credentials: "same-origin",
    ...init,
    headers: {
      "X-Requested-With": "fetch",
      ...(init?.headers || {}),
    },
  });
}

async function asJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await request(path, init);
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) {
    throw new ApiError(res.status, data?.error || res.statusText);
  }
  return data as T;
}

export const api = {
  get: <T>(path: string) => asJSON<T>(path),

  post: <T>(path: string, body?: unknown) =>
    asJSON<T>(path, {
      method: "POST",
      headers: body !== undefined ? { "Content-Type": "application/json" } : {},
      body: body !== undefined ? JSON.stringify(body) : undefined,
    }),

  patch: <T>(path: string, body: unknown) =>
    asJSON<T>(path, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    }),

  del: <T>(path: string) => asJSON<T>(path, { method: "DELETE" }),

  upload: <T>(path: string, file: File | Blob, name: string) => {
    const fd = new FormData();
    fd.append("file", file);
    fd.append("name", name);
    return asJSON<T>(path, { method: "POST", body: fd });
  },
};
