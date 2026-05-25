// Renders one ApiCall: the request line, a colour-coded status, latency,
// and a scrollable pretty-printed JSON body.

import { Box, Text } from "ink";
import type { ApiCall } from "../api/client.js";

export function statusColor(status: number): string {
  if (status === 0) return "red";
  if (status >= 500) return "red";
  if (status >= 400) return "yellow";
  if (status >= 200 && status < 300) return "green";
  return "white";
}

interface ResponsePanelProps {
  call?: ApiCall;
  /** First body line to show (scroll offset). */
  scroll: number;
  /** How many body lines fit. */
  height: number;
  hint?: string;
}

export function ResponsePanel({ call, scroll, height, hint }: ResponsePanelProps) {
  if (!call) {
    return (
      <Box flexDirection="column">
        <Text dimColor>{hint ?? "Select an endpoint and press Enter to fire it."}</Text>
      </Box>
    );
  }

  const body =
    call.data === undefined
      ? call.error ?? "(no body)"
      : JSON.stringify(call.data, null, 2);
  const lines = body.split("\n");
  const visible = lines.slice(scroll, scroll + height);
  const more = lines.length - (scroll + visible.length);

  return (
    <Box flexDirection="column">
      <Box>
        <Text bold>{call.method} </Text>
        <Text>{call.path}</Text>
      </Box>
      <Box>
        <Text color={statusColor(call.status)} bold>
          {call.status === 0 ? "NO RESPONSE" : `HTTP ${call.status}`}
        </Text>
        <Text dimColor>{`  ${call.ms}ms`}</Text>
        {call.error ? <Text color="red">{`  ${call.error}`}</Text> : null}
      </Box>
      <Box marginTop={1} flexDirection="column">
        {visible.map((line, i) => (
          <Text key={scroll + i}>{line || " "}</Text>
        ))}
        {more > 0 ? (
          <Text dimColor>{`... +${more} more line(s) — scroll with [ and ]`}</Text>
        ) : null}
      </Box>
    </Box>
  );
}
