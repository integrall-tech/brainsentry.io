// Headless validation runner — `npm run validate`. Authenticates, runs
// every memory scenario, prints a plain-text report and exits non-zero
// on any failure, so it can gate CI.

import { BrainSentryClient } from "./api/client.js";
import {
  detectCapabilities,
  formatCapabilities,
} from "./capabilities.js";
import { loadConfig } from "./config.js";
import { allScenarios } from "./scenarios/index.js";
import { runAll } from "./scenarios/runner.js";

const useColor = process.stdout.isTTY;
const c = {
  green: (s: string) => (useColor ? `\x1b[32m${s}\x1b[0m` : s),
  red: (s: string) => (useColor ? `\x1b[31m${s}\x1b[0m` : s),
  dim: (s: string) => (useColor ? `\x1b[2m${s}\x1b[0m` : s),
  bold: (s: string) => (useColor ? `\x1b[1m${s}\x1b[0m` : s),
};

export async function runHeadless(): Promise<number> {
  const cfg = loadConfig();
  const client = new BrainSentryClient(cfg);

  process.stdout.write(c.bold("brain-sentry explorer — API validation\n"));
  process.stdout.write(
    c.dim(`  target ${cfg.baseUrl}  ·  tenant ${cfg.tenantId}\n\n`),
  );

  const authCall =
    cfg.authMode === "login"
      ? await client.login(cfg.email, cfg.password)
      : await client.demoLogin();
  if (!authCall.ok || !client.token) {
    process.stderr.write(
      c.red(
        `authentication failed: ${authCall.error ?? `HTTP ${authCall.status}`}\n`,
      ),
    );
    process.stderr.write(
      c.dim("is the backend running? check BS_BASE_URL (see .env.example)\n"),
    );
    return 1;
  }

  // Detect capabilities once so scenarios that need pgvector / falkordb /
  // an LLM key can skip honestly when the backend doesn't have them.
  const caps = await detectCapabilities(client);
  process.stdout.write(c.dim(`  caps:  ${formatCapabilities(caps)}\n\n`));

  const summary = await runAll(allScenarios, client, caps, (e) => {
    if (e.type === "scenario-start") {
      process.stdout.write(c.bold(`${e.title}\n`));
    } else if (e.type === "scenario-skip") {
      process.stdout.write(
        c.bold(`${e.title}`) +
          c.dim(`  · skipped (missing ${e.missing.join(", ")})\n\n`),
      );
    } else if (e.type === "step-result") {
      const r = e.result;
      const mark =
        r.status === "pass"
          ? c.green("  ✓ ")
          : r.status === "fail"
            ? c.red("  ✗ ")
            : c.dim("  ⊘ ");
      process.stdout.write(`${mark}${r.name}${c.dim(`  (${r.ms}ms)`)}\n`);
      if (r.message && r.status === "fail") {
        process.stdout.write(c.red(`      ${r.message}\n`));
      }
    } else if (e.type === "scenario-end") {
      if (e.result.skipped === 0) process.stdout.write("\n");
    }
  });

  const headline =
    summary.totalFailed === 0
      ? c.green(c.bold("ALL CHECKS PASSED"))
      : c.red(c.bold(`${summary.totalFailed} CHECK(S) FAILED`));
  const skippedNote =
    summary.totalSkipped > 0
      ? c.dim(`  ·  ${summary.totalSkipped} skipped`)
      : "";
  process.stdout.write(
    `${headline}  ${summary.totalPassed}/${summary.totalSteps} steps` +
      skippedNote +
      c.dim(`  ·  ${summary.ms}ms\n`),
  );

  return summary.totalFailed === 0 ? 0 : 1;
}
