import { test, expect } from "../fixtures/auth.fixture";
import { mockAdminApis } from "../helpers/admin-mocks";
import { ROUTES } from "../helpers/constants";

// Completes the memory CRUD coverage: memory-crud.spec.ts covers create +
// maintenance dialogs; here we cover update (PUT) and delete (DELETE).
test.describe("Memory Update & Delete", () => {
  test.beforeEach(async ({ authenticatedPage }) => {
    await mockAdminApis(authenticatedPage);
    await authenticatedPage.goto(ROUTES.memories);
  });

  test("edits an existing memory and the list refreshes", async ({ authenticatedPage: page }) => {
    await expect(page.getByText("Autenticacao com refresh token", { exact: true })).toBeVisible();

    // First card is mem-auth — open the edit dialog
    await page.getByRole("button", { name: "Editar" }).first().click();
    await expect(page.getByRole("heading", { name: "Editar Memória" })).toBeVisible();
    await expect(page.locator("#summary")).toHaveValue("Autenticacao com refresh token");

    await page.locator("#summary").fill("Autenticacao com refresh token v2");
    await page.locator("#content").fill("Fluxo atualizado: rotação de refresh token a cada uso.");

    const putResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/memories/") && r.request().method() === "PUT"
    );
    await page.getByRole("button", { name: /^Salvar$/i }).click();
    await putResponse;

    // Dialog closes and the refreshed list shows the updated summary
    await expect(page.getByRole("heading", { name: "Editar Memória" })).not.toBeVisible();
    await expect(page.getByText("Autenticacao com refresh token v2")).toBeVisible();
    await expect(page.getByText("Autenticacao com refresh token", { exact: true })).not.toBeVisible();
  });

  test("deletes a memory and removes it from the list", async ({ authenticatedPage: page }) => {
    await expect(page.getByText("Autenticacao com refresh token", { exact: true })).toBeVisible();

    // Accept the native confirm() shown by handleDelete
    page.on("dialog", (dialog) => dialog.accept());

    const deleteResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/memories/") && r.request().method() === "DELETE"
    );
    await page.getByRole("button", { name: "Excluir" }).first().click();
    await deleteResponse;

    // Success toast (network anchor keeps us inside the 5s auto-dismiss window)
    await expect(page.getByText("Memória excluída")).toBeVisible();

    // Card removed from the grid; other memories remain
    await expect(page.getByText("Autenticacao com refresh token", { exact: true })).not.toBeVisible();
    await expect(page.getByText("Busca semantica no admin", { exact: true })).toBeVisible();
  });
});
