// Validation view. Runs every memory scenario against the live backend
// and renders per-step pass/fail as results stream in.

import { Box, Text } from "ink";
import Spinner from "ink-spinner";
import { useEffect, useState } from "react";
import type { BrainSentryClient } from "../api/client.js";
import {
  type Capabilities,
  detectCapabilities,
  formatCapabilities,
} from "../capabilities.js";
import { allScenarios } from "../scenarios/index.js";
import {
  runAll,
  type RunSummary,
  type StepResult,
} from "../scenarios/runner.js";

interface ScenarioProgress {
  id: string;
  title: string;
  steps: StepResult[];
  finished: boolean;
  skippedFor?: string[];
}

interface ValidationProps {
  client: BrainSentryClient;
}

export function Validation({ client }: ValidationProps) {
  const [progress, setProgress] = useState<ScenarioProgress[]>([]);
  const [summary, setSummary] = useState<RunSummary | undefined>(undefined);
  const [caps, setCaps] = useState<Capabilities | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const detected = await detectCapabilities(client);
      if (cancelled) return;
      setCaps(detected);
      await runAll(allScenarios, client, detected, (e) => {
        if (cancelled) return;
        setProgress((prev) => {
          const next = prev.map((p) => ({ ...p, steps: [...p.steps] }));
          if (e.type === "scenario-start") {
            next.push({ id: e.id, title: e.title, steps: [], finished: false });
          } else if (e.type === "scenario-skip") {
            next.push({
              id: e.id,
              title: e.title,
              steps: [],
              finished: true,
              skippedFor: e.missing,
            });
          } else if (e.type === "step-result") {
            const sc = next.find((p) => p.id === e.scenarioId);
            if (sc) sc.steps.push(e.result);
          } else if (e.type === "scenario-end") {
            const sc = next.find((p) => p.id === e.result.id);
            if (sc) sc.finished = true;
          }
          return next;
        });
        if (e.type === "done") setSummary(e.summary);
      });
    })();
    return () => {
      cancelled = true;
    };
  }, [client]);

  return (
    <Box flexDirection="column" paddingX={1}>
      {caps ? (
        <Box marginBottom={1}>
          <Text dimColor>caps:  {formatCapabilities(caps)}</Text>
        </Box>
      ) : null}

      {progress.map((sc) => (
        <Box key={sc.id} flexDirection="column" marginBottom={1}>
          <Box>
            <Text bold color="cyan">
              {sc.title}
            </Text>
            {sc.skippedFor ? (
              <Text dimColor>
                {`  · skipped (missing ${sc.skippedFor.join(", ")})`}
              </Text>
            ) : null}
          </Box>
          {sc.steps.map((step, i) => {
            const color =
              step.status === "pass"
                ? "green"
                : step.status === "fail"
                  ? "red"
                  : "gray";
            const mark =
              step.status === "pass"
                ? "  ✓ "
                : step.status === "fail"
                  ? "  ✗ "
                  : "  ⊘ ";
            return (
              <Box key={i} flexDirection="column">
                <Box>
                  <Text color={color}>{mark}</Text>
                  <Text>{step.name}</Text>
                  <Text dimColor>{`  ${step.ms}ms`}</Text>
                </Box>
                {step.message && step.status === "fail" ? (
                  <Text color="red">{`      ${step.message}`}</Text>
                ) : null}
              </Box>
            );
          })}
        </Box>
      ))}

      {summary ? (
        <Box
          borderStyle="round"
          borderColor={summary.totalFailed === 0 ? "green" : "red"}
          paddingX={1}
        >
          <Text bold color={summary.totalFailed === 0 ? "green" : "red"}>
            {summary.totalFailed === 0 ? "ALL CHECKS PASSED" : "FAILURES FOUND"}
          </Text>
          <Text>
            {`  ${summary.totalPassed}/${summary.totalSteps} passed`}
          </Text>
          {summary.totalSkipped > 0 ? (
            <Text dimColor>{`  · ${summary.totalSkipped} skipped`}</Text>
          ) : null}
          <Text dimColor>{`  · ${summary.ms}ms`}</Text>
        </Box>
      ) : (
        <Text color="yellow">
          <Spinner type="dots" /> running validation scenarios...
        </Text>
      )}
    </Box>
  );
}
