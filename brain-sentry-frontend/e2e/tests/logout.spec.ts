import { test, expect } from "@playwright/test";
import { LoginPage } from "../pages/login.page";
import { mockAdminApis, mockAuthApis, forceE2ELanguage } from "../helpers/admin-mocks";
import { DEMO_EMAIL, DEMO_PASSWORD, ROUTES, STORAGE_KEYS } from "../helpers/constants";

// NOTE: this spec logs in through the real LoginPage instead of the
// authenticatedPage fixture: the fixture seeds localStorage via
// addInitScript, which re-runs on EVERY page load and would re-inject the
// token after logout — making the "protected route redirects" assertion
// meaningless.
test.describe("Logout", () => {
  test.beforeEach(async ({ page }) => {
    await forceE2ELanguage(page);
    await mockAuthApis(page);
    await mockAdminApis(page);
  });

  test("logs out, clears storage and protects routes again", async ({ page }) => {
    // Login
    const loginPage = new LoginPage(page);
    await loginPage.goto();
    await loginPage.login(DEMO_EMAIL, DEMO_PASSWORD);
    await expect(page).toHaveURL(/\/app\/dashboard/);

    const tokenBefore = await page.evaluate(
      (key) => localStorage.getItem(key),
      STORAGE_KEYS.token
    );
    expect(tokenBefore).not.toBeNull();

    // Click the logout button — sidebar on desktop, menu overlay on mobile.
    // Branch by viewport (md breakpoint = 768px), not by isVisible(): the
    // aside is display:none under md:, and isVisible() doesn't auto-wait,
    // so it races the initial render on slower browsers.
    const isMobile = (page.viewportSize()?.width ?? 1280) < 768;
    if (isMobile) {
      await page.locator("div.md\\:hidden").first().locator("button").last().click();
      await page.getByTestId("mobile-logout").click();
    } else {
      await page.getByTestId("sidebar-logout").click();
    }

    // Redirected to /login
    await expect(page).toHaveURL(/\/login/);

    // localStorage cleared (token + user removed)
    const tokenAfter = await page.evaluate(
      (key) => localStorage.getItem(key),
      STORAGE_KEYS.token
    );
    expect(tokenAfter).toBeNull();
    const userAfter = await page.evaluate(
      (key) => localStorage.getItem(key),
      STORAGE_KEYS.user
    );
    expect(userAfter).toBeNull();

    // Protected route redirects back to /login
    await page.goto(ROUTES.dashboard);
    await expect(page).toHaveURL(/\/login/);
  });
});
