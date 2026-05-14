import { test, expect } from "../fixtures/auth.fixture";
import { mockAdminApis } from "../helpers/admin-mocks";
import { ROUTES } from "../helpers/constants";

test.describe("Models (tier routing + doctor)", () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test.beforeEach(async ({ authenticatedPage }) => {
    await mockAdminApis(authenticatedPage);
    await authenticatedPage.goto(ROUTES.models);
  });

  test("renders the routing snapshot table with one row per tier", async ({ authenticatedPage }) => {
    await expect(authenticatedPage.getByTestId("models-page")).toBeVisible();
    for (const tier of ["utility", "reasoning", "deep", "subagent"]) {
      await expect(authenticatedPage.getByTestId(`models-row-${tier}`)).toBeVisible();
    }
    await expect(authenticatedPage.getByTestId("models-row-utility")).toContainText("openai/gpt-4o-mini");
    await expect(authenticatedPage.getByTestId("models-row-reasoning")).toContainText("config-tier");
  });

  test("running the doctor reveals failures with classification and hint", async ({ authenticatedPage }) => {
    await authenticatedPage.getByTestId("models-run-doctor").click();
    await expect(authenticatedPage.getByTestId("models-doctor")).toBeVisible();
    await expect(authenticatedPage.getByTestId("models-doctor-aggregate")).toContainText("Algum provedor falhou");
    const subagent = authenticatedPage.getByTestId("models-probe-subagent");
    await expect(subagent).toHaveAttribute("data-status", "fail");
    await expect(subagent).toContainText("model_not_found");
    await expect(subagent).toContainText("phantom/model-x");
    await expect(subagent).toContainText("check for typos");
  });

  test("doctor shows the passing tiers as PASS", async ({ authenticatedPage }) => {
    await authenticatedPage.getByTestId("models-run-doctor").click();
    await expect(authenticatedPage.getByTestId("models-probe-utility")).toHaveAttribute("data-status", "ok");
    await expect(authenticatedPage.getByTestId("models-probe-reasoning")).toHaveAttribute("data-status", "ok");
  });

  test("guide drawer carries business-focused copy", async ({ authenticatedPage }) => {
    await authenticatedPage.getByTestId("screen-help-trigger").click();
    const drawer = authenticatedPage.getByTestId("screen-help-drawer");
    await expect(drawer).toBeVisible();
    await expect(drawer).toContainText("Roteamento de Modelos");
    await expect(drawer).toContainText("Qual IA está sendo usada para o quê");
  });

  test("refresh re-issues the snapshot request", async ({ authenticatedPage }) => {
    let calls = 0;
    await authenticatedPage.route("**/v1/models", async (route) => {
      calls += 1;
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          snapshot: [
            { tier: "utility", model: "u", source: "config-tier" },
            { tier: "reasoning", model: "r", source: "config-tier" },
            { tier: "deep", model: "d", source: "config-tier" },
            { tier: "subagent", model: "s", source: "tier-default" },
          ],
        }),
      });
    });
    await authenticatedPage.reload();
    const before = calls;
    await authenticatedPage.getByTestId("models-refresh").click();
    await expect.poll(() => calls).toBeGreaterThan(before);
  });
});
