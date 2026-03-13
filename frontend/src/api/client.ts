import { API_BASE_URL, SESSION_STORAGE_KEY } from "@/app/constants";
import type { ApiErrorEnvelope } from "@/api/types";

type RequestOptions = RequestInit & {
  timeoutMs?: number;
  auth?: boolean;
};

let unauthorizedHandler: (() => void) | null = null;

export class ApiError extends Error {
  status: number;
  code: string;
  details?: unknown;
  requestId?: string;

  constructor(
    status: number,
    code: string,
    message: string,
    details?: unknown,
    requestId?: string,
  ) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
    this.requestId = requestId;
  }
}

export function setUnauthorizedHandler(handler: (() => void) | null) {
  unauthorizedHandler = handler;
}

function getSessionToken() {
  try {
    const raw = localStorage.getItem(SESSION_STORAGE_KEY);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as { token?: string };
    return parsed.token ?? null;
  } catch {
    return null;
  }
}

export function buildWebSocketURL(path: string, token?: string | null) {
  const base = new URL(API_BASE_URL);
  base.protocol = base.protocol === "https:" ? "wss:" : "ws:";
  const prefix = base.pathname.endsWith("/")
    ? base.pathname.slice(0, -1)
    : base.pathname;
  base.pathname = `${prefix}${path.startsWith("/") ? path : `/${path}`}`;
  if (token) {
    base.searchParams.set("token", token);
  }
  return base.toString();
}

export function buildAssetURL(path: string, token?: string | null) {
  const base = new URL(API_BASE_URL);
  const prefix = base.pathname.endsWith("/")
    ? base.pathname.slice(0, -1)
    : base.pathname;
  const resolved = new URL(
    `${prefix}${path.startsWith("/") ? path : `/${path}`}`,
    base,
  );
  if (token) {
    resolved.searchParams.set("token", token);
  }
  return resolved.toString();
}

async function parseBody(response: Response) {
  const text = await response.text();
  if (!text) {
    return null;
  }

  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}

function stripHTML(input: string) {
  return input.replace(/<[^>]*>/g, " ").replace(/\s+/g, " ").trim();
}

export async function apiRequest<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const controller = new AbortController();
  const timeoutMs = options.timeoutMs ?? 8000;
  const requestId = crypto.randomUUID();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);

  const headers = new Headers(options.headers ?? {});
  headers.set("Accept", "application/json");
  headers.set("X-Request-Id", requestId);
  if (
    !headers.has("Content-Type") &&
    options.body &&
    !(options.body instanceof FormData)
  ) {
    headers.set("Content-Type", "application/json");
  }

  if (options.auth !== false) {
    const token = getSessionToken();
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }
  }

  try {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      ...options,
      headers,
      signal: controller.signal,
    });

    const body = await parseBody(response);
    const responseRequestId = response.headers.get("X-Request-Id") ?? requestId;

    if (!response.ok) {
      const payload = (body ?? {}) as ApiErrorEnvelope;
      const code = payload.error?.code ?? "API_ERROR";
      const fallbackMessage =
        response.status >= 500
          ? "El servicio no está disponible en este momento."
          : `HTTP ${response.status}`;
      const message =
        payload.error?.message ??
        (typeof body === "string"
          ? stripHTML(body).slice(0, 240) || fallbackMessage
          : fallbackMessage);

      if (response.status === 401 && unauthorizedHandler) {
        unauthorizedHandler();
      }

      throw new ApiError(
        response.status,
        code,
        message,
        payload.error?.details,
        payload.requestId ?? responseRequestId,
      );
    }

    return body as T;
  } catch (error) {
    if (error instanceof ApiError) {
      throw error;
    }
    if (error instanceof DOMException && error.name === "AbortError") {
      throw new ApiError(408, "TIMEOUT", "La solicitud tardó demasiado.");
    }
    throw new ApiError(500, "NETWORK_ERROR", "No pudimos conectar con la API.");
  } finally {
    clearTimeout(timeout);
  }
}
