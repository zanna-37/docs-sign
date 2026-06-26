export class ApiError extends Error {
  status: number;
  // The parsed response body, so callers can read structured payloads on non-2xx replies
  // (e.g. the conflict list returned with 409 from a restore).
  data: unknown;
  constructor(status: number, message: string, data?: unknown) {
    super(message);
    this.status = status;
    this.data = data;
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
    throw new ApiError(res.status, data?.error || res.statusText, data);
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

// Byte counts for an in-flight upload.
export interface UploadProgress {
  loaded: number;
  total: number;
}

// uploadWithProgress posts a single file as multipart/form-data and reports byte-level upload
// progress. It uses XMLHttpRequest because fetch exposes no upload progress events; the request
// shape, credentials, CSRF header and error mapping mirror api.upload / asJSON.
export function uploadWithProgress<T>(
  path: string,
  file: File | Blob,
  name: string,
  onProgress?: (p: UploadProgress) => void,
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const fd = new FormData();
    fd.append("file", file);
    fd.append("name", name);

    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api" + path);
    // Same-origin request: cookies ride along by default. The custom header satisfies the
    // server's CSRF guard (browsers can't set it cross-origin without a denied preflight).
    xhr.setRequestHeader("X-Requested-With", "fetch");

    if (onProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) onProgress({ loaded: e.loaded, total: e.total });
      };
    }
    xhr.onload = () => {
      let data: { error?: string } | null = null;
      try {
        data = xhr.responseText ? JSON.parse(xhr.responseText) : null;
      } catch {
        data = null;
      }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(data as T);
      } else {
        reject(new ApiError(xhr.status, data?.error || xhr.statusText, data));
      }
    };
    xhr.onerror = () => reject(new ApiError(0, "Network error"));
    xhr.onabort = () => reject(new ApiError(0, "Upload aborted"));
    xhr.send(fd);
  });
}
