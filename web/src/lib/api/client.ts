export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields?: Record<string, string>,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export async function request<T = void>(path: string, init: RequestInit = {}): Promise<T> {
  const url = path.startsWith("/api/") ? path : `/api/v1${path}`;
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const response = await fetch(url, { ...init, headers, credentials: "same-origin" });
  if (response.redirected || response.headers.get("content-type")?.includes("text/html")) {
    throw new ApiError(401, "session_expired", "Your session has expired. Sign in to continue.");
  }
  const text = await response.text();
  let body: unknown;
  try {
    body = text ? JSON.parse(text) : undefined;
  } catch {
    body = undefined;
  }
  if (!response.ok) {
    const envelope = body as
      | { error?: { code?: string; message?: string; fields?: Record<string, string> } }
      | undefined;
    throw new ApiError(
      response.status,
      envelope?.error?.code ?? "request_failed",
      envelope?.error?.message ??
        (text && text.length < 240 ? text : "Something went wrong. Please try again."),
      envelope?.error?.fields,
    );
  }
  return body as T;
}

const body = (value?: unknown) => (value === undefined ? undefined : JSON.stringify(value));
export const api = {
  get: <T>(path: string, signal?: AbortSignal) => request<T>(path, { signal }),
  post: <T = void>(path: string, value?: unknown, signal?: AbortSignal) =>
    request<T>(path, { method: "POST", body: body(value), signal }),
  put: <T = void>(path: string, value?: unknown) =>
    request<T>(path, { method: "PUT", body: body(value) }),
  patch: <T = void>(path: string, value?: unknown) =>
    request<T>(path, { method: "PATCH", body: body(value) }),
  delete: <T = void>(path: string) => request<T>(path, { method: "DELETE" }),
};
export const mediaPath = (id: string) => encodeURIComponent(id);
