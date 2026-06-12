import { test, expect } from "../fixtures/auth.fixture";
import { mockAdminApis } from "../helpers/admin-mocks";

// Coverage for the 4 governance pages that previously had no E2E specs:
// Policies, Events, Reasoning (abduction) and Provenance (PROV-O export).
test.describe("Governance Pages", () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test.beforeEach(async ({ authenticatedPage }) => {
    await mockAdminApis(authenticatedPage);
  });

  test("policies: lists, creates a policy and enforces against a decision", async ({ authenticatedPage: page }) => {
    await page.goto("/app/policies");

    await expect(page.getByRole("heading", { name: "Políticas", exact: true })).toBeVisible();
    // Seeded policy from mocks
    await expect(page.getByText("Confiança mínima").first()).toBeVisible();
    await expect(page.getByText("Políticas (1)")).toBeVisible();

    // Create a new policy through the form
    await page.getByLabel("Nome", { exact: true }).fill("Bloquear categoria sensível");
    await page.getByLabel("Descrição").fill("Categoria proibida para decisões automáticas");
    await page.getByLabel("Severidade").selectOption("error");
    await page.getByLabel("Tipo de regra").selectOption("category_blocked");

    const createResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/policies") && r.request().method() === "POST"
    );
    await page.getByRole("button", { name: "Criar política" }).click();
    await createResponse;

    await expect(page.getByText("Bloquear categoria sensível")).toBeVisible();
    await expect(page.getByText("Políticas (2)")).toBeVisible();

    // Enforce against a decision and inspect violations
    await page.getByPlaceholder("decisionId (UUID)").fill("dec-001");
    const enforceResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/policies/enforce") && r.request().method() === "POST"
    );
    await page.getByRole("button", { name: "Verificar" }).click();
    await enforceResponse;

    await expect(page.getByText(/Compliant:/i)).toBeVisible();
    await expect(page.getByText("Confiança 0.62 abaixo do mínimo 0.7")).toBeVisible();
  });

  test("events: lists, registers a new event and filters by type", async ({ authenticatedPage: page }) => {
    await page.goto("/app/events");

    await expect(page.getByRole("heading", { name: "Eventos", exact: true })).toBeVisible();
    await expect(page.getByText("Deploy do backend v2")).toBeVisible();
    await expect(page.getByText("Eventos (2)")).toBeVisible();

    // Register a new event. Note: the "Registrar" tab toggle and the form
    // submit button share the same label — scope to the <form> to disambiguate.
    await page.getByPlaceholder("meeting, deploy, incident...").fill("meeting");
    await page.getByLabel("Título").fill("Kickoff do projeto");

    const recordResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/events") && r.request().method() === "POST"
    );
    await page.locator("form").getByRole("button", { name: "Registrar" }).click();
    await recordResponse;

    await expect(page.getByText("Kickoff do projeto")).toBeVisible();
    await expect(page.getByText("Eventos (3)")).toBeVisible();

    // Filter by eventType — mock filters server-side
    const filterResponse = page.waitForResponse(
      (r) =>
        r.url().includes("/v1/events") &&
        r.url().includes("eventType=incident") &&
        r.request().method() === "GET"
    );
    await page.getByPlaceholder("tipo", { exact: true }).fill("incident");
    await filterResponse;

    await expect(page.getByText("Incidente de latência")).toBeVisible();
    await expect(page.getByText("Deploy do backend v2")).not.toBeVisible();
    await expect(page.getByText("Eventos (1)")).toBeVisible();
  });

  test("reasoning: fetches latest decision and runs abduction", async ({ authenticatedPage: page }) => {
    await page.goto("/app/reasoning");

    await expect(page.getByRole("heading", { name: "Raciocínio" })).toBeVisible();

    // Pull latest decision id into the form
    const decisionsResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/decisions") && r.request().method() === "GET"
    );
    await page.getByRole("button", { name: "Pegar mais recente" }).click();
    await decisionsResponse;
    await expect(page.getByPlaceholder("UUID")).toHaveValue("dec-001");

    // Run abductive reasoning
    const abduceResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/reasoning/abduce") && r.request().method() === "POST"
    );
    await page.getByRole("button", { name: "Abduzir" }).click();
    await abduceResponse;

    await expect(page.getByText("Decisão alvo")).toBeVisible();
    await expect(page.getByText("Hipóteses (2)")).toBeVisible();
    await expect(page.getByText("Pressão por reduzir blast radius em produção")).toBeVisible();
    await expect(page.getByText("Canary release do serviço de busca")).toBeVisible();

    // Expand the top hypothesis (scope to <main> so we don't catch sidebar group chevrons)
    await page.locator("main button:has(svg.lucide-chevron-down)").first().click();
    await expect(page.getByText("Evidências")).toBeVisible();
    await expect(page.getByText("Incidente recente de latência")).toBeVisible();
  });

  test("provenance: previews PROV-O export in Turtle and JSON-LD", async ({ authenticatedPage: page }) => {
    await page.goto("/app/provenance");

    await expect(page.getByRole("heading", { name: "Proveniência" })).toBeVisible();
    await expect(page.getByText("Proveniência W3C PROV-O")).toBeVisible();

    // Preview Turtle ("Turtle" exact — avoids the "Baixar Turtle (.ttl)" button)
    const turtleResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/export/provenance") && r.url().includes("format=turtle")
    );
    await page.getByRole("button", { name: "Turtle", exact: true }).click();
    await turtleResponse;
    await expect(page.locator("pre")).toContainText("@prefix prov:");

    // Preview JSON-LD
    const jsonldResponse = page.waitForResponse(
      (r) => r.url().includes("/v1/export/provenance") && r.url().includes("format=jsonld")
    );
    await page.getByRole("button", { name: "JSON-LD", exact: true }).click();
    await jsonldResponse;
    await expect(page.locator("pre")).toContainText("@context");
  });
});
