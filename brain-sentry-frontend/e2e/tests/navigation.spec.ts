import { test, expect } from "../fixtures/auth.fixture";
import { mockAdminApis } from "../helpers/admin-mocks";
import { Sidebar } from "../pages/sidebar.page";
import { ROUTES } from "../helpers/constants";

test.describe("Navigation", () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test.beforeEach(async ({ authenticatedPage }) => {
    await mockAdminApis(authenticatedPage);
    await authenticatedPage.goto(ROUTES.dashboard);
  });

  test("renders every admin item in the sidebar", async ({ authenticatedPage }) => {
    const sidebar = new Sidebar(authenticatedPage);

    for (const item of sidebar.getAllNavItems()) {
      await expect(sidebar.getNavItem(item)).toBeVisible();
    }

    await expect(sidebar.userEmail).toContainText("demo@example.com");
  });

  test("switches between key admin routes", async ({ authenticatedPage }) => {
    const sidebar = new Sidebar(authenticatedPage);

    await sidebar.navigateTo("Memórias");
    await expect(authenticatedPage).toHaveURL(/\/app\/memories/);

    await sidebar.navigateTo("Relacionamentos");
    await expect(authenticatedPage).toHaveURL(/\/app\/relationships/);

    await sidebar.navigateTo("Configurações");
    await expect(authenticatedPage).toHaveURL(/\/app\/configuration/);

    await sidebar.navigateTo("Dashboard");
    await expect(authenticatedPage).toHaveURL(/\/app\/dashboard/);
  });

  test("sidebar collapses to rail mode and persists across reload", async ({ authenticatedPage }) => {
    const sidebar = authenticatedPage.getByTestId("sidebar");
    await expect(sidebar).toHaveAttribute("data-collapsed", "false");

    // Group label is visible when expanded
    await expect(authenticatedPage.getByRole("button", { name: "Conhecimento" })).toBeVisible();

    // Collapse
    await authenticatedPage.getByTestId("sidebar-toggle").click();
    await expect(sidebar).toHaveAttribute("data-collapsed", "true");

    // Group labels gone in rail mode
    await expect(authenticatedPage.getByRole("button", { name: "Conhecimento" })).toHaveCount(0);

    // Item icons still navigate (use title fallback)
    await authenticatedPage.getByRole("button", { name: "Memórias" }).click();
    await expect(authenticatedPage).toHaveURL(/\/app\/memories/);

    // Reload preserves collapsed state via localStorage
    await authenticatedPage.reload();
    await expect(authenticatedPage.getByTestId("sidebar")).toHaveAttribute("data-collapsed", "true");

    // Toggle back to expanded for downstream tests
    await authenticatedPage.getByTestId("sidebar-toggle").click();
    await expect(authenticatedPage.getByTestId("sidebar")).toHaveAttribute("data-collapsed", "false");
  });

  test("navigates to new Cognee pages", async ({ authenticatedPage }) => {
    const sidebar = new Sidebar(authenticatedPage);

    const checks: Array<[string, RegExp]> = [
      ["Console", /\/app\/console/],
      ["Traços de Agente", /\/app\/traces/],
      ["Lab de Extração", /\/app\/extraction/],
      ["Ontologia", /\/app\/ontology/],
      ["Cache de Sessão", /\/app\/session-cache/],
      ["Ações & Leases", /\/app\/actions/],
      ["Sincronização Mesh", /\/app\/mesh/],
      ["Busca em Lote", /\/app\/batch-search/],
    ];

    for (const [item, urlRegex] of checks) {
      await sidebar.navigateTo(item);
      await expect(authenticatedPage).toHaveURL(urlRegex);
    }
  });
});
