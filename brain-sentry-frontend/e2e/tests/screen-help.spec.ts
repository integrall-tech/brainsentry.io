import { test, expect } from "../fixtures/auth.fixture";
import { mockAdminApis } from "../helpers/admin-mocks";
import { Sidebar } from "../pages/sidebar.page";
import { ROUTES } from "../helpers/constants";

test.describe("Screen Help (Guide)", () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test.beforeEach(async ({ authenticatedPage }) => {
    await authenticatedPage.addInitScript(() => {
      // Ensure a clean "never visited" state for the badge tests.
      localStorage.removeItem("brainsentry.help.visited");
    });
    await mockAdminApis(authenticatedPage);
    await authenticatedPage.goto(ROUTES.dashboard);
  });

  test("floating guide button is visible on every screen", async ({ authenticatedPage }) => {
    const trigger = authenticatedPage.getByTestId("screen-help-trigger");
    await expect(trigger).toBeVisible();
    await expect(trigger).toContainText("Guia");
  });

  test("opens drawer with Objetivo/Como funciona/Fluxo sections", async ({ authenticatedPage }) => {
    await authenticatedPage.getByTestId("screen-help-trigger").click();

    const drawer = authenticatedPage.getByTestId("screen-help-drawer");
    await expect(drawer).toBeVisible();

    // Header shows the screen title (dashboard → "Visão geral")
    await expect(drawer.getByRole("heading", { name: "Visão geral" })).toBeVisible();

    // Standard sections rendered
    await expect(drawer.getByText("Objetivo", { exact: true })).toBeVisible();
    await expect(drawer.getByText("Problema que resolve", { exact: true })).toBeVisible();
    await expect(drawer.getByText("Como funciona", { exact: true })).toBeVisible();
    await expect(drawer.getByText("Fluxo sugerido", { exact: true })).toBeVisible();
  });

  test("closes via the X button and via overlay click", async ({ authenticatedPage }) => {
    const trigger = authenticatedPage.getByTestId("screen-help-trigger");
    await trigger.click();
    const drawer = authenticatedPage.getByTestId("screen-help-drawer");
    await expect(drawer).toBeVisible();

    // Close with X
    await drawer.getByRole("button", { name: "Fechar" }).click();
    await expect(drawer).toHaveCount(0);

    // Reopen and close via overlay
    await trigger.click();
    await expect(drawer).toBeVisible();
    await authenticatedPage.getByTestId("screen-help-overlay").click({ force: true });
    await expect(drawer).toHaveCount(0);
  });

  test("content changes per route: decisions screen has its own copy", async ({ authenticatedPage }) => {
    const sidebar = new Sidebar(authenticatedPage);
    await sidebar.navigateTo("Decisões");
    await expect(authenticatedPage).toHaveURL(/\/app\/decisions/);

    await authenticatedPage.getByTestId("screen-help-trigger").click();

    const drawer = authenticatedPage.getByTestId("screen-help-drawer");
    await expect(drawer.getByRole("heading", { name: "Decisões" })).toBeVisible();
    // Distinctive phrase from the decisions help copy
    await expect(drawer.getByText(/registro das escolhas do time/i)).toBeVisible();
  });

  test("shows 'new' badge on first visit and removes it after opening the drawer", async ({ authenticatedPage }) => {
    await expect(authenticatedPage.getByTestId("screen-help-new-badge")).toBeVisible();

    await authenticatedPage.getByTestId("screen-help-trigger").click();
    await expect(authenticatedPage.getByTestId("screen-help-drawer")).toBeVisible();
    await authenticatedPage.getByTestId("screen-help-drawer").getByRole("button", { name: "Fechar" }).click();

    await expect(authenticatedPage.getByTestId("screen-help-new-badge")).toHaveCount(0);
  });

  test("graph views have dedicated guides", async ({ authenticatedPage }) => {
    const sidebar = new Sidebar(authenticatedPage);

    await sidebar.navigateTo("Grafo Global");
    await authenticatedPage.getByTestId("screen-help-trigger").click();
    await expect(
      authenticatedPage.getByTestId("screen-help-drawer").getByRole("heading", { name: "Grafo Global" }),
    ).toBeVisible();
    await authenticatedPage.getByTestId("screen-help-drawer").getByRole("button", { name: "Fechar" }).click();

    await sidebar.navigateTo("Ego-grafo");
    await authenticatedPage.getByTestId("screen-help-trigger").click();
    await expect(
      authenticatedPage.getByTestId("screen-help-drawer").getByRole("heading", { name: "Ego-grafo" }),
    ).toBeVisible();
  });
});
