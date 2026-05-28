// Root TUI component. Authenticates on mount, then toggles between the
// interactive Explorer and the Validation runner.

import { Box, Text, useApp, useInput } from "ink";
import Spinner from "ink-spinner";
import { useEffect, useState } from "react";
import type { BrainSentryClient } from "../api/client.js";
import { Explorer } from "./Explorer.js";
import { Validation } from "./Validation.js";

type AuthState = "pending" | "ok" | "error";
type Mode = "explore" | "validate";

interface AppProps {
  client: BrainSentryClient;
}

export function App({ client }: AppProps) {
  const { exit } = useApp();
  const [auth, setAuth] = useState<AuthState>("pending");
  const [authMsg, setAuthMsg] = useState("");
  const [mode, setMode] = useState<Mode>("explore");
  // Bumped each time validation is launched, to remount (re-run) the view.
  const [runId, setRunId] = useState(0);

  const cfg = client.config;

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const call =
        cfg.authMode === "login"
          ? await client.login(cfg.email, cfg.password)
          : await client.demoLogin();
      if (cancelled) return;
      if (call.ok && client.token) {
        setAuth("ok");
      } else {
        setAuth("error");
        setAuthMsg(call.error ?? `authentication failed (HTTP ${call.status})`);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client, cfg]);

  useInput((input, key) => {
    if (input === "q" || (key.ctrl && input === "c")) {
      exit();
      return;
    }
    if (auth !== "ok") return;
    if (input === "v") {
      setRunId((n) => n + 1);
      setMode("validate");
    } else if (input === "e" || key.escape) {
      setMode("explore");
    }
  });

  return (
    <Box flexDirection="column">
      {/* Header */}
      <Box justifyContent="space-between" paddingX={1}>
        <Text bold color="magenta">
          brain-sentry explorer
        </Text>
        <Text dimColor>
          {cfg.baseUrl} · tenant {cfg.tenantId.slice(0, 8)} ·{" "}
          {auth === "ok" ? (
            <Text color="green">authenticated</Text>
          ) : auth === "pending" ? (
            <Text color="yellow">connecting</Text>
          ) : (
            <Text color="red">auth failed</Text>
          )}
        </Text>
      </Box>

      {/* Body */}
      {auth === "pending" ? (
        <Box paddingX={1} paddingY={1}>
          <Text color="yellow">
            <Spinner type="dots" />{" "}
            {cfg.authMode === "login"
              ? "logging in..."
              : "requesting a demo token..."}
          </Text>
        </Box>
      ) : auth === "error" ? (
        <Box flexDirection="column" paddingX={1} paddingY={1}>
          <Text color="red">Could not authenticate: {authMsg}</Text>
          <Text dimColor>
            Check the backend is running and BS_BASE_URL is correct
            (see .env.example). Press q to quit.
          </Text>
        </Box>
      ) : mode === "explore" ? (
        <Explorer client={client} />
      ) : (
        <Validation key={runId} client={client} />
      )}

      {/* Footer */}
      <Box paddingX={1}>
        {auth !== "ok" ? (
          <Text dimColor>q quit</Text>
        ) : mode === "explore" ? (
          <Text dimColor>
            ↑↓ select · Enter fire · [ ] scroll · r reset context · v run
            validation · q quit
          </Text>
        ) : (
          <Text dimColor>e back to explorer · v re-run · q quit</Text>
        )}
      </Box>
    </Box>
  );
}
