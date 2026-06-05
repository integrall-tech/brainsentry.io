#!/usr/bin/env tsx
// Entry point. `--validate` runs the headless validation suite and exits
// with its status; otherwise the interactive TUI is rendered.
//
// The ink TUI (and its `render`/App imports) are loaded LAZILY, only on
// the interactive path. The headless `--validate` mode is what CI and
// smoke-test.sh use — it must run even if the TUI deps (ink) aren't
// fully installed, so we never import ink at module top level.

const args = process.argv.slice(2);

if (args.includes("--validate")) {
  const { runHeadless } = await import("./validate.js");
  runHeadless()
    .then((code) => process.exit(code))
    .catch((err) => {
      process.stderr.write(`${err instanceof Error ? err.stack : err}\n`);
      process.exit(1);
    });
} else if (args.includes("--help") || args.includes("-h")) {
  process.stdout.write(
    "brain-sentry-explorer — example client for the brainsentry.io memory API\n\n" +
      "  npm start        launch the interactive TUI explorer\n" +
      "  npm run validate run the validation suite (headless, CI-friendly)\n\n" +
      "Configuration: copy .env.example to .env (see that file for keys).\n",
  );
  process.exit(0);
} else {
  const { render } = await import("ink");
  const { App } = await import("./components/App.js");
  const { BrainSentryClient } = await import("./api/client.js");
  const { loadConfig } = await import("./config.js");
  render(<App client={new BrainSentryClient(loadConfig())} />);
}
