// BrainSentryClient — a thin, fully-typed HTTP client for the
// brainsentry.io API. Every call returns an ApiCall record (status,
// latency, body, error) instead of throwing, so both the interactive
// explorer and the validation runner can render results uniformly.

import axios, { type AxiosInstance } from "axios";
import type { Config } from "../config.js";

export type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";

export interface ApiCall<T = unknown> {
  method: HttpMethod;
  path: string;
  /** HTTP status, or 0 when the request never reached the server. */
  status: number;
  ok: boolean;
  /** Round-trip latency in milliseconds. */
  ms: number;
  data?: T;
  /** Human-readable failure reason (transport error or error body). */
  error?: string;
  /** Wall-clock timestamp the call completed. */
  at: number;
}

export interface RequestOptions {
  body?: unknown;
  query?: Record<string, string | number | boolean | undefined>;
}

export class BrainSentryClient {
  readonly config: Config;
  private http: AxiosInstance;
  token: string | null = null;
  /** Rolling log of every call made — the explorer renders the tail. */
  calls: ApiCall<unknown>[] = [];

  constructor(config: Config) {
    this.config = config;
    this.http = axios.create({
      baseURL: config.baseUrl,
      timeout: 30_000,
      // Capture every status — never throw on 4xx/5xx, the caller decides.
      validateStatus: () => true,
    });
  }

  private buildQuery(query?: RequestOptions["query"]): string {
    if (!query) return "";
    const params = new URLSearchParams();
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined) params.set(k, String(v));
    }
    const s = params.toString();
    return s ? `?${s}` : "";
  }

  async request<T = unknown>(
    method: HttpMethod,
    path: string,
    opts: RequestOptions = {},
  ): Promise<ApiCall<T>> {
    const fullPath = path + this.buildQuery(opts.query);
    const headers: Record<string, string> = {
      "X-Tenant-ID": this.config.tenantId,
    };
    if (this.token) headers.Authorization = `Bearer ${this.token}`;
    if (opts.body !== undefined) headers["Content-Type"] = "application/json";

    const started = performance.now();
    let call: ApiCall<T>;
    try {
      const res = await this.http.request({
        method,
        url: fullPath,
        headers,
        data: opts.body,
      });
      const ms = Math.round(performance.now() - started);
      const ok = res.status >= 200 && res.status < 300;
      call = {
        method,
        path: fullPath,
        status: res.status,
        ok,
        ms,
        data: res.data as T,
        error: ok ? undefined : extractError(res.data, res.status),
        at: Date.now(),
      };
    } catch (err) {
      const ms = Math.round(performance.now() - started);
      call = {
        method,
        path: fullPath,
        status: 0,
        ok: false,
        ms,
        error: describeTransportError(err),
        at: Date.now(),
      };
    }
    this.calls.push(call);
    if (this.calls.length > 200) this.calls.shift();
    return call;
  }

  /** POST /v1/auth/demo — issues a token for the seeded demo user. */
  async demoLogin(): Promise<ApiCall> {
    const call = await this.request("POST", "/v1/auth/demo");
    this.adoptToken(call);
    return call;
  }

  /** POST /v1/auth/login — email/password authentication. */
  async login(email: string, password: string): Promise<ApiCall> {
    const call = await this.request("POST", "/v1/auth/login", {
      body: { email, password },
    });
    this.adoptToken(call);
    return call;
  }

  private adoptToken(call: ApiCall): void {
    if (!call.ok || !call.data || typeof call.data !== "object") return;
    const data = call.data as Record<string, unknown>;
    if (typeof data.token === "string") this.token = data.token;
  }
}

// Pull a readable message out of an error body shaped like {error|message}.
function extractError(data: unknown, status: number): string {
  if (data && typeof data === "object") {
    const d = data as Record<string, unknown>;
    if (typeof d.error === "string") return d.error;
    if (typeof d.message === "string") return d.message;
  }
  return `HTTP ${status}`;
}

// Render an axios/network error as a non-empty string. When the URL uses
// a hostname like "localhost" and Node retries v6 → v4, AxiosError.message
// can come back empty — `.code` ("ECONNREFUSED", "ETIMEDOUT", ...) is the
// reliable signal there.
function describeTransportError(err: unknown): string {
  if (err && typeof err === "object") {
    const e = err as { message?: unknown; code?: unknown };
    if (typeof e.message === "string" && e.message !== "") return e.message;
    if (typeof e.code === "string" && e.code !== "") return `network ${e.code}`;
  }
  if (err instanceof Error && err.message) return err.message;
  return "request failed (no message)";
}
