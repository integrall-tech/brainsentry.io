import { test, expect } from "@playwright/test";
import { mockAdminApis, MOCK_AUTH } from "../helpers/admin-mocks";
import { ROUTES, STORAGE_KEYS, E2E_LANGUAGE } from "../helpers/constants";

// RBAC: a USER (non-ADMIN) hitting an admin-only action gets a 403 from the
// backend. The UI must surface the error (toast) and keep working — it must
// not crash or silently pretend the action succeeded.
// The happy path with ADMIN is already covered in admin-pages.spec.ts.
const USER_AUTH = {
  ...MOCK_AUTH,
  user: {
    ...MOCK_AUTH.user,
    id: "user-basic",
    email: "ana@example.com",
    name: "Ana Analyst",
    roles: ["USER"],
  },
};

const FORBIDDEN_MESSAGE = "Permissão negada: requer papel ADMIN";

test.describe("RBAC — USER role", () => {
  test.beforeEach(async ({ page }) => {
    // Seed a session with role USER (no ADMIN)
    await page.addInitScript(
      ({ auth, keys, lang }) => {
        localStorage.setItem(keys.token, auth.token);
        localStorage.setItem(keys.user, JSON.stringify(auth.user));
        localStorage.setItem(keys.tenantId, auth.tenantId);
        localStorage.setItem(keys.language, lang);
      },
      { auth: USER_AUTH, keys: STORAGE_KEYS, lang: E2E_LANGUAGE }
    );

    await mockAdminApis(page);

    // Registered AFTER mockAdminApis so it takes precedence (Playwright
    // checks the most recently registered route first). Only the admin
    // mutation is forbidden; GET list still works via mockAdminApis.
    await page.route("**/v1/tenants", async (route) => {
      if (route.request().method() === "POST") {
        return route.fulfill({
          status: 403,
          headers: { "access-control-allow-origin": "*", "content-type": "application/json" },
          body: JSON.stringify({ message: FORBIDDEN_MESSAGE }),
        });
      }
      return route.fallback();
    });
  });

  test("shows backend 403 error on admin action without breaking the UI", async ({ page }) => {
    await page.goto(ROUTES.tenants);

    // Page loads normally for the USER role
    await expect(page.getByText("BrainSentry Core")).toBeVisible();

    // Attempt the admin-only action: create tenant
    await page.getByRole("button", { name: /Novo Tenant/i }).click();
    await page.getByPlaceholder("Minha Organização").fill("Tenant Proibido");

    const forbiddenResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/tenants") && r.request().method() === "POST" && r.status() === 403
    );
    await page.getByRole("button", { name: /Criar Tenant/i }).click();
    await forbiddenResponse;

    // Error toast with the backend message (network anchor above keeps us
    // inside the 5s toast auto-dismiss window)
    await expect(page.getByText(FORBIDDEN_MESSAGE)).toBeVisible();

    // UI did not break: the create dialog stays open (only closes on success)
    await expect(page.getByRole("heading", { name: "Novo Tenant" })).toBeVisible();

    // And the forbidden tenant was NOT added to the list
    await page.getByRole("button", { name: /Cancelar/i }).click();
    await expect(page.getByText("BrainSentry Core")).toBeVisible();
    await expect(page.getByText("Tenant Proibido")).not.toBeVisible();
  });
});
