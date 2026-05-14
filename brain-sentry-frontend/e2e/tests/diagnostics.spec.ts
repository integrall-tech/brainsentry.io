import { test, expect } from "../fixtures/auth.fixture";
import { mockAdminApis } from "../helpers/admin-mocks";
import { ROUTES } from "../helpers/constants";

test.describe("Diagnostics (doctor)", () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test.beforeEach(async ({ authenticatedPage }) => {
    await mockAdminApis(authenticatedPage);
    await authenticatedPage.goto(ROUTES.diagnostics);
  });

  test("renders the page header and refresh button", async ({ authenticatedPage }) => {
    await expect(authenticatedPage.getByTestId("diagnostics-page")).toBeVisible();
    await expect(authenticatedPage.getByTestId("diagnostics-refresh")).toBeVisible();
  });

  test("renders an aggregate summary with the correct status from the mock", async ({ authenticatedPage }) => {
    const summary = authenticatedPage.getByTestId("diagnostics-summary");
    await expect(summary).toBeVisible();
    await expect(authenticatedPage.getByTestId("diagnostics-aggregate-status")).toContainText("Atenção");
    await expect(authenticatedPage.getByTestId("diagnostics-stat-ok")).toHaveText("4");
    await expect(authenticatedPage.getByTestId("diagnostics-stat-warn")).toHaveText("1");
    await expect(authenticatedPage.getByTestId("diagnostics-stat-fail")).toHaveText("0");
  });

  test("lists every check returned by the API with name, message and status colour", async ({ authenticatedPage }) => {
    const list = authenticatedPage.getByTestId("diagnostics-checks");
    await expect(list).toBeVisible();
    for (const name of ["postgres", "postgres-ping", "redis", "falkordb", "openrouter"]) {
      await expect(authenticatedPage.getByTestId(`diagnostics-check-${name}`)).toBeVisible();
    }
    await expect(
      authenticatedPage.getByTestId("diagnostics-check-redis"),
    ).toHaveAttribute("data-status", "warn");
    await expect(
      authenticatedPage.getByTestId("diagnostics-check-postgres"),
    ).toHaveAttribute("data-status", "ok");
  });

  test("shows hint text for failing checks", async ({ authenticatedPage }) => {
    const redis = authenticatedPage.getByTestId("diagnostics-check-redis");
    await expect(redis).toContainText("rate-limit + caching degraded without redis");
  });

  test("clicking refresh re-issues the diagnostics request", async ({ authenticatedPage }) => {
    let calls = 0;
    await authenticatedPage.route("**/v1/diagnostics", async (route) => {
      calls += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "ok",
          generated_at: new Date().toISOString(),
          duration_ms: 10,
          checks: [
            { name: "postgres", status: "ok", severity: "critical", message: "ok", duration_ms: 1 },
          ],
          summary: { ok: 1, warn: 0, fail: 0, skip: 0 },
        }),
      });
    });
    await authenticatedPage.reload();
    await expect(authenticatedPage.getByTestId("diagnostics-aggregate-status")).toContainText("Tudo certo");
    const before = calls;
    await authenticatedPage.getByTestId("diagnostics-refresh").click();
    await expect.poll(() => calls).toBeGreaterThan(before);
  });

  test("guide drawer for diagnostics opens with business-focused content", async ({ authenticatedPage }) => {
    await authenticatedPage.getByTestId("screen-help-trigger").click();
    const drawer = authenticatedPage.getByTestId("screen-help-drawer");
    await expect(drawer).toBeVisible();
    await expect(drawer).toContainText("Diagnóstico do Sistema");
    await expect(drawer).toContainText("Tudo está funcionando?");
  });
});
