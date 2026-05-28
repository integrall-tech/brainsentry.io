// Scenario runner. A Scenario is an ordered list of Steps that share a
// mutable context object. Each step either returns normally (pass) or
// throws (fail). The runner records per-step results plus the HTTP calls
// each step produced, and emits live events so the TUI can render
// progress as it happens.

import type { ApiCall, BrainSentryClient } from "../api/client.js";
import {
  type Capabilities,
  type Capability,
  missingCapabilities,
} from "../capabilities.js";
import { AssertionError } from "./assert.js";

export interface StepContext {
  client: BrainSentryClient;
  /** Cross-step scratch space — IDs created earlier, counts, etc. */
  vars: Record<string, unknown>;
}

export interface Step {
  name: string;
  run: (ctx: StepContext) => Promise<void>;
}

export interface Scenario {
  id: string;
  title: string;
  description: string;
  steps: Step[];
  /** Backend capabilities this scenario depends on; missing → skip. */
  requires?: readonly Capability[];
}

export type StepStatus = "pass" | "fail" | "skip";

export interface StepResult {
  name: string;
  status: StepStatus;
  ms: number;
  message?: string;
  /** HTTP calls the step issued, for drill-down in the UI. */
  calls: ApiCall[];
}

export interface ScenarioResult {
  id: string;
  title: string;
  results: StepResult[];
  passed: number;
  failed: number;
  skipped: number;
  /** Capabilities the scenario needed but the backend doesn't have. */
  skippedFor?: Capability[];
}

export interface RunSummary {
  scenarios: ScenarioResult[];
  totalSteps: number;
  totalPassed: number;
  totalFailed: number;
  totalSkipped: number;
  ms: number;
  capabilities?: Capabilities;
}

// Live events for the interactive runner.
export type RunEvent =
  | { type: "scenario-start"; id: string; title: string }
  | { type: "scenario-skip"; id: string; title: string; missing: Capability[] }
  | { type: "step-result"; scenarioId: string; result: StepResult }
  | { type: "scenario-end"; result: ScenarioResult }
  | { type: "done"; summary: RunSummary };

export async function runScenario(
  scenario: Scenario,
  client: BrainSentryClient,
  capabilities?: Capabilities,
  onEvent?: (e: RunEvent) => void,
): Promise<ScenarioResult> {
  // Pre-flight: if any required capability is missing, skip the whole
  // scenario and mark each step skipped (so the count is honest).
  const missing = capabilities
    ? missingCapabilities(capabilities, scenario.requires)
    : [];
  if (missing.length > 0) {
    onEvent?.({
      type: "scenario-skip",
      id: scenario.id,
      title: scenario.title,
      missing,
    });
    const results: StepResult[] = scenario.steps.map((s) => ({
      name: s.name,
      status: "skip",
      ms: 0,
      calls: [],
      message: `requires ${missing.join(", ")}`,
    }));
    const scenarioResult: ScenarioResult = {
      id: scenario.id,
      title: scenario.title,
      results,
      passed: 0,
      failed: 0,
      skipped: results.length,
      skippedFor: missing,
    };
    onEvent?.({ type: "scenario-end", result: scenarioResult });
    return scenarioResult;
  }

  onEvent?.({ type: "scenario-start", id: scenario.id, title: scenario.title });
  const ctx: StepContext = { client, vars: {} };
  const results: StepResult[] = [];

  for (const step of scenario.steps) {
    const before = client.calls.length;
    const started = performance.now();
    let result: StepResult;
    try {
      await step.run(ctx);
      result = {
        name: step.name,
        status: "pass",
        ms: Math.round(performance.now() - started),
        calls: client.calls.slice(before),
      };
    } catch (err) {
      result = {
        name: step.name,
        status: "fail",
        ms: Math.round(performance.now() - started),
        message: describeError(err),
        calls: client.calls.slice(before),
      };
    }
    results.push(result);
    onEvent?.({ type: "step-result", scenarioId: scenario.id, result });
  }

  const passed = results.filter((r) => r.status === "pass").length;
  const failed = results.filter((r) => r.status === "fail").length;
  const scenarioResult: ScenarioResult = {
    id: scenario.id,
    title: scenario.title,
    results,
    passed,
    failed,
    skipped: 0,
  };
  onEvent?.({ type: "scenario-end", result: scenarioResult });
  return scenarioResult;
}

export async function runAll(
  scenarios: Scenario[],
  client: BrainSentryClient,
  capabilities?: Capabilities,
  onEvent?: (e: RunEvent) => void,
): Promise<RunSummary> {
  const started = performance.now();
  const scenarioResults: ScenarioResult[] = [];
  for (const scenario of scenarios) {
    scenarioResults.push(
      await runScenario(scenario, client, capabilities, onEvent),
    );
  }
  const totalPassed = scenarioResults.reduce((n, s) => n + s.passed, 0);
  const totalFailed = scenarioResults.reduce((n, s) => n + s.failed, 0);
  const totalSkipped = scenarioResults.reduce((n, s) => n + s.skipped, 0);
  const summary: RunSummary = {
    scenarios: scenarioResults,
    totalSteps: totalPassed + totalFailed + totalSkipped,
    totalPassed,
    totalFailed,
    totalSkipped,
    ms: Math.round(performance.now() - started),
    capabilities,
  };
  onEvent?.({ type: "done", summary });
  return summary;
}

function describeError(err: unknown): string {
  if (err instanceof AssertionError) return err.message;
  if (err instanceof Error) return `${err.name}: ${err.message}`;
  return String(err);
}
