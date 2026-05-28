// Single point that combines every scenario set the suite knows about.
// validate.ts and the Validation TUI both import from here, so adding a
// new scenario set is one import line away.

import { memoryScenarios } from "./memory-scenarios.js";
import { salesScenarios } from "./sales-scenarios.js";
import type { Scenario } from "./runner.js";

export { memoryScenarios, salesScenarios };

export const allScenarios: Scenario[] = [
  ...memoryScenarios,
  ...salesScenarios,
];
