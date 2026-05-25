// Runtime configuration. Resolved once at startup from environment
// variables, which Node loads from .env via process.loadEnvFile().

export interface Config {
  baseUrl: string;
  tenantId: string;
  authMode: "demo" | "login";
  email: string;
  password: string;
}

// Load .env if present. Node 20.12+ exposes process.loadEnvFile; it throws
// when the file is absent, which is fine — env vars may be set directly.
try {
  process.loadEnvFile();
} catch {
  // no .env file — fall back to the process environment as-is
}

function env(name: string, fallback = ""): string {
  const v = process.env[name];
  return v === undefined || v === "" ? fallback : v;
}

export function loadConfig(): Config {
  const authMode = env("BS_AUTH_MODE", "demo") === "login" ? "login" : "demo";
  return {
    baseUrl: env("BS_BASE_URL", "http://localhost:8081/api").replace(/\/$/, ""),
    tenantId: env("BS_TENANT_ID", "a9f814d2-4dae-41f3-851b-8aa3d4706561"),
    authMode,
    email: env("BS_EMAIL"),
    password: env("BS_PASSWORD"),
  };
}
