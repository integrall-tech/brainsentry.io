// Validation view. Runs every memory scenario against the live backend
// and renders per-step pass/fail as results stream in.

import { Box, Text } from "ink";
import Spinner from "ink-spinner";
import { useEffect, useState } from "react";
import type { BrainSentryClient } from "../api/client.js";
import { memoryScenarios } from "../scenarios/memory-scenarios.js";
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
}

interface ValidationProps {
  client: BrainSentryClient;
}

export function Validation({ client }: ValidationProps) {
  const [progress, setProgress] = useState<ScenarioProgress[]>([]);
  const [summary, setSummary] = useState<RunSummary | undefined>(undefined);

  useEffect(() => {
    let cancelled = false;
    void runAll(memoryScenarios, client, (e) => {
      if (cancelled) return;
      setProgress((prev) => {
        const next = prev.map((p) => ({ ...p, steps: [...p.steps] }));
        if (e.type === "scenario-start") {
          next.push({ id: e.id, title: e.title, steps: [], finished: false });
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
    return () => {
      cancelled = true;
    };
  }, [client]);

  return (
    <Box flexDirection="column" paddingX={1}>
      {progress.map((sc) => (
        <Box key={sc.id} flexDirection="column" marginBottom={1}>
          <Text bold color="cyan">
            {sc.title}
          </Text>
          {sc.steps.map((step, i) => (
            <Box key={i} flexDirection="column">
              <Box>
                <Text color={step.status === "pass" ? "green" : "red"}>
                  {step.status === "pass" ? "  ✓ " : "  ✗ "}
                </Text>
                <Text>{step.name}</Text>
                <Text dimColor>{`  ${step.ms}ms`}</Text>
              </Box>
              {step.message ? (
                <Text color="red">{`      ${step.message}`}</Text>
              ) : null}
            </Box>
          ))}
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
            {`  ${summary.totalPassed}/${summary.totalSteps} steps passed`}
          </Text>
          <Text dimColor>{`  ${summary.ms}ms total`}</Text>
        </Box>
      ) : (
        <Text color="yellow">
          <Spinner type="dots" /> running validation scenarios...
        </Text>
      )}
    </Box>
  );
}
