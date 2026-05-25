// Interactive endpoint explorer. Browse the catalog, fire any entry, see
// the live request/response. IDs captured from responses are chained into
// the context so dependent endpoints (GET/{id}, relationships, ...) just
// work without typing UUIDs.

import { Box, Text, useInput } from "ink";
import Spinner from "ink-spinner";
import { useMemo, useState } from "react";
import type { ApiCall, BrainSentryClient } from "../api/client.js";
import { CATALOG, catalogGroups, type ExplorerCtx } from "../catalog.js";
import { ResponsePanel } from "./ResponsePanel.js";

const RESPONSE_HEIGHT = 16;

interface ExplorerProps {
  client: BrainSentryClient;
}

export function Explorer({ client }: ExplorerProps) {
  const groups = useMemo(() => catalogGroups(), []);
  const [selected, setSelected] = useState(0);
  const [ctx, setCtx] = useState<ExplorerCtx>({});
  const [call, setCall] = useState<ApiCall | undefined>(undefined);
  const [scroll, setScroll] = useState(0);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<string | undefined>(undefined);

  const endpoint = CATALOG[selected];
  const missing = (endpoint.needs ?? []).filter((k) => !ctx[k]);

  async function fire() {
    if (busy) return;
    if (missing.length > 0) {
      setNote(
        `Needs ${missing.join(", ")} in context first — ` +
          `fire a Create endpoint to capture it.`,
      );
      setCall(undefined);
      return;
    }
    setNote(undefined);
    setBusy(true);
    const req = endpoint.build(ctx);
    const result = await client.request(endpoint.method, req.path, {
      body: req.body,
      query: req.query,
    });
    if (result.ok && endpoint.capture) {
      const next = { ...ctx };
      endpoint.capture(result.data, next);
      setCtx(next);
    }
    setCall(result);
    setScroll(0);
    setBusy(false);
  }

  useInput((input, key) => {
    if (busy) return;
    if (key.upArrow || input === "k") {
      setSelected((s) => Math.max(0, s - 1));
    } else if (key.downArrow || input === "j") {
      setSelected((s) => Math.min(CATALOG.length - 1, s + 1));
    } else if (key.return) {
      void fire();
    } else if (input === "]") {
      setScroll((s) => s + 4);
    } else if (input === "[") {
      setScroll((s) => Math.max(0, s - 4));
    } else if (input === "r") {
      setCtx({});
      setNote("Context cleared.");
    }
  });

  let globalIndex = -1;

  return (
    <Box flexDirection="column">
      <Box>
        {/* Catalog */}
        <Box
          flexDirection="column"
          width="46%"
          borderStyle="round"
          borderColor="gray"
          paddingX={1}
        >
          {groups.map((g) => (
            <Box key={g.group} flexDirection="column">
              <Text bold color="cyan">
                {g.group}
              </Text>
              {g.endpoints.map((ep) => {
                globalIndex += 1;
                const idx = globalIndex;
                const active = idx === selected;
                const unmet = (ep.needs ?? []).some((k) => !ctx[k]);
                return (
                  <Text
                    key={ep.id}
                    color={active ? "black" : unmet ? "gray" : undefined}
                    backgroundColor={active ? "cyan" : undefined}
                  >
                    {active ? "> " : "  "}
                    {ep.method.padEnd(6)}
                    {ep.label.replace(/^\w+ /, "")}
                  </Text>
                );
              })}
            </Box>
          ))}
        </Box>

        {/* Request / response */}
        <Box
          flexDirection="column"
          flexGrow={1}
          borderStyle="round"
          borderColor="gray"
          paddingX={1}
          marginLeft={1}
        >
          <Text dimColor>{endpoint.summary}</Text>
          <Box marginTop={1} flexDirection="column">
            {busy ? (
              <Text color="yellow">
                <Spinner type="dots" /> firing {endpoint.label}...
              </Text>
            ) : (
              <ResponsePanel
                call={call}
                scroll={scroll}
                height={RESPONSE_HEIGHT}
                hint={note ?? "Press Enter to fire the selected endpoint."}
              />
            )}
            {!busy && note ? <Text color="yellow">{note}</Text> : null}
          </Box>
        </Box>
      </Box>

      {/* Context bar */}
      <Box marginTop={0} paddingX={1}>
        <Text dimColor>context </Text>
        {Object.keys(ctx).length === 0 ? (
          <Text dimColor>(empty — fire a Create endpoint to populate)</Text>
        ) : (
          <Text>
            {Object.entries(ctx)
              .filter(([, v]) => v)
              .map(([k, v]) => `${k}=${(v as string).slice(0, 8)}`)
              .join("  ")}
          </Text>
        )}
      </Box>
    </Box>
  );
}
