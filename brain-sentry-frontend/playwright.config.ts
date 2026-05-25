import { defineConfig, devices } from "@playwright/test";

// PW_SLOWMO=<ms> adds a delay between every Playwright action so the
// headed run is watchable in real time. 0 (default) keeps CI fast.
// Suggested values: 250 (comfortable demo), 500 (presentation), 1000 (slow narration).
const slowMo = parseInt(process.env.PW_SLOWMO || "0", 10);

// When slowMo is in use, individual actions take longer; bump per-test
// timeouts so a 60-step test isn't capped at 30s.
const timeoutMultiplier = slowMo > 0 ? Math.max(3, Math.ceil(slowMo / 100)) : 1;

export default defineConfig({
  testDir: "./e2e/tests",
  testIgnore: /real-.*\.spec\.ts/,
  globalSetup: "./e2e/global-setup.ts",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? "github" : [["html", { open: "never" }], ["list"]],
  timeout: 30_000 * timeoutMultiplier,
  expect: { timeout: 10_000 * timeoutMultiplier },

  use: {
    baseURL: "http://127.0.0.1:4601",
    locale: "pt-BR",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    launchOptions: { slowMo },
  },

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },
    {
      name: "mobile-chrome",
      use: { ...devices["Pixel 5"] },
    },
  ],

  webServer: {
    command: "npm run dev -- --host 127.0.0.1 --port 4601 --strictPort",
    url: "http://127.0.0.1:4601",
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
