#!/usr/bin/env tsx
// Entry point. `--validate` runs the headless validation suite and exits
// with its status; otherwise the interactive TUI is rendered.

import { render } from "ink";
import { BrainSentryClient } from "./api/client.js";
import { App } from "./components/App.js";
import { loadConfig } from "./config.js";
import { runHeadless } from "./validate.js";

const args = process.argv.slice(2);

if (args.includes("--validate")) {
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
  const client = new BrainSentryClient(loadConfig());
  render(<App client={client} />);
}
