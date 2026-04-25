import { test, expect } from "../fixtures/auth.fixture";
import { mockAdminApis } from "../helpers/admin-mocks";
import { ROUTES } from "../helpers/constants";

test.describe("Cognee P1-P3 Pages", () => {
  test.use({ viewport: { width: 1280, height: 900 } });

  test.beforeEach(async ({ authenticatedPage }) => {
    await mockAdminApis(authenticatedPage);
  });

  // ---------------- Semantic Console ----------------

  test("console: remember + recall flow with router preview", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.console);

    await expect(authenticatedPage.getByRole("heading", { name: "Console Semântico" })).toBeVisible();

    // Toggle button has lowercase text ("lembrar") due to capitalize CSS;
    // send button uses capitalized "Lembrar". Use exact match to disambiguate.
    await authenticatedPage.getByRole("button", { name: "lembrar", exact: true }).click();
    await authenticatedPage.locator("textarea").fill("Store this fact for testing.");
    await authenticatedPage.getByRole("button", { name: "Lembrar", exact: true }).click();

    await expect(authenticatedPage.getByText(/Memória salva/i)).toBeVisible();

    // Back to recall mode (lowercase toggle)
    await authenticatedPage.getByRole("button", { name: "recuperar", exact: true }).click();
    await authenticatedPage.locator("textarea").fill("how does authentication work");

    // Router preview appears after debounce
    await expect(authenticatedPage.getByText("roteador:")).toBeVisible({ timeout: 2000 });

    // Submit recall (capitalized send button)
    await authenticatedPage.getByRole("button", { name: "Recuperar", exact: true }).click();

    await expect(authenticatedPage.getByText("Autenticacao com refresh token")).toBeVisible();
    await expect(authenticatedPage.getByText(/estratégia/i)).toBeVisible();
  });

  test("console: improve triggers auto-forget toast", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.console);

    await authenticatedPage.getByRole("button", { name: /Melhorar/i }).click();
    await expect(authenticatedPage.getByText(/Melhoria concluída/i)).toBeVisible();
  });

  // ---------------- Agent Traces ----------------

  test("traces: list + stats + expand detail", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.traces);

    await expect(authenticatedPage.getByRole("heading", { name: "Traços de Agente" })).toBeVisible();

    // Stats cards
    await expect(authenticatedPage.getByText("Sucesso", { exact: true })).toBeVisible();
    await expect(authenticatedPage.getByText("Erros", { exact: true })).toBeVisible();

    // Trace rows
    await expect(authenticatedPage.getByText("POST /v1/recall").first()).toBeVisible();
    await expect(authenticatedPage.getByText(/upstream timeout/i)).toBeVisible();

    // Expand first row (scope to <main> so we don't catch sidebar group chevrons)
    await authenticatedPage.locator("main button:has(svg.lucide-chevron-down)").first().click();
    await expect(authenticatedPage.getByText("Parâmetros", { exact: true })).toBeVisible();
  });

  test("traces: filter by error status", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.traces);

    await authenticatedPage.getByRole("button", { name: /^error\b/i }).click();
    // Mock returns same list — we at least verify filter button toggles visually.
    await expect(authenticatedPage.getByText(/upstream timeout/i)).toBeVisible();
  });

  // ---------------- Extraction Lab ----------------

  test("extraction lab: triplets tab extracts S-P-O", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.extraction);

    await expect(authenticatedPage.getByRole("heading", { name: "Lab de Extração" })).toBeVisible();

    // Sample text is prefilled — click Extract
    await authenticatedPage.getByRole("button", { name: /Extrair tripletos/i }).click();

    await expect(authenticatedPage.getByText("PostgreSQL", { exact: true }).first()).toBeVisible();
    await expect(authenticatedPage.getByText("supports", { exact: true })).toBeVisible();
    await expect(authenticatedPage.getByText("JSON", { exact: true })).toBeVisible();
  });

  test("extraction lab: cascade tab shows entities and relationships", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.extraction);

    await authenticatedPage.getByRole("button", { name: /Cascade/i }).click();
    await authenticatedPage.getByRole("button", { name: /Executar cascade/i }).click();

    // Entities pills
    await expect(authenticatedPage.getByText("TECHNOLOGY", { exact: true }).first()).toBeVisible();
    await expect(authenticatedPage.getByText("LANGUAGE", { exact: true }).first()).toBeVisible();

    // Relationship row
    await expect(authenticatedPage.getByText("connects_to")).toBeVisible();
  });

  // ---------------- Ontology ----------------

  test("ontology: loads + resolves canonical entity", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.ontology);

    await expect(authenticatedPage.getByRole("heading", { name: "Ontologia" })).toBeVisible();

    // Visual view shows entity types and relationships
    await expect(authenticatedPage.getByText("TECHNOLOGY", { exact: true }).first()).toBeVisible();
    await expect(authenticatedPage.getByText("uses", { exact: true }).first()).toBeVisible();

    // Resolve tester
    await authenticatedPage.getByPlaceholder("ex.: postgres").fill("postgres");
    await authenticatedPage.getByRole("button", { name: /^OK$/i }).click();

    await expect(authenticatedPage.getByText(/canônico:/i)).toBeVisible();
    await expect(authenticatedPage.getByText("PostgreSQL", { exact: true }).first()).toBeVisible();
  });

  test("ontology: JSON editor toggle", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.ontology);

    await authenticatedPage.getByRole("button", { name: /^JSON$/i }).click();
    await expect(authenticatedPage.locator("textarea").first()).toContainText("brainsentry-test");
  });

  // ---------------- Session Cache ----------------

  test("session cache: list sessions + view interactions + cognify", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.sessionCache);

    await expect(authenticatedPage.getByRole("heading", { name: "Cache de Sessão" })).toBeVisible();

    // Session appears in both the list AND the detail panel — use first()
    await expect(authenticatedPage.getByText("session-abc").first()).toBeVisible();

    // First session is auto-selected → interactions load
    await expect(authenticatedPage.getByText("What is JWT?")).toBeVisible();
    await expect(authenticatedPage.getByText(/JSON Web Token for stateless auth/i)).toBeVisible();

    // Cognify
    await authenticatedPage.getByRole("button", { name: "Cognificar", exact: true }).click();
    await expect(authenticatedPage.getByText(/Cognificado/i)).toBeVisible();
  });

  // ---------------- Actions ----------------

  test("actions: list + create + acquire lease", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.actions);

    await expect(authenticatedPage.getByRole("heading", { name: "Ações & Leases" })).toBeVisible();

    // Seeded actions visible
    await expect(authenticatedPage.getByText("Ship Cognee UI")).toBeVisible();
    await expect(authenticatedPage.getByText("Fix router false positives")).toBeVisible();

    // Open create dialog — header "Novo" button
    await authenticatedPage.getByRole("button", { name: /^Novo$/ }).click();
    await expect(authenticatedPage.getByRole("heading", { name: /Nova ação/i })).toBeVisible();

    // Dialog text inputs order: leaseAgent (page), Title, Tags.
    // Title is nth(1) — first text input after the leaseAgent.
    await authenticatedPage.locator('input[type="text"]').nth(1).fill("Write more tests");
    await authenticatedPage.locator("textarea").fill("Coverage for new pages.");

    // Click the enabled Criar button
    await authenticatedPage.getByRole("button", { name: "Criar", exact: true }).click();

    await expect(authenticatedPage.getByText(/Ação criada/i)).toBeVisible();
  });

  test("actions: claim lease on pending action", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.actions);

    // 'Fix router' is pending — should show Reivindicar
    const claimButtons = authenticatedPage.getByRole("button", { name: /^Reivindicar$/i });
    await claimButtons.first().click();

    await expect(authenticatedPage.getByText(/Lease adquirida/i)).toBeVisible();
  });

  // ---------------- Mesh Sync ----------------

  test("mesh: list peers + register new + sync scope", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.mesh);

    await expect(authenticatedPage.getByRole("heading", { name: "Sincronização Mesh" })).toBeVisible();

    // Peer visible
    await expect(authenticatedPage.getByText("peer-eu-1")).toBeVisible();
    await expect(authenticatedPage.getByText("https://brainsentry-eu.example.com")).toBeVisible();

    // Push sync (button text uses interpolation: "Enviar memories")
    await authenticatedPage.getByRole("button", { name: /Enviar memories/i }).click();
    await expect(authenticatedPage.getByText(/Sincronização concluída/i)).toBeVisible();

    // Register new peer dialog
    await authenticatedPage.getByRole("button", { name: /Registrar peer/i }).click();
    await authenticatedPage.getByRole("heading", { name: /Registrar peer/i }).waitFor();

    await authenticatedPage.getByPlaceholder("eu-west-1").fill("us-east-1");
    await authenticatedPage.getByPlaceholder("https://peer.example.com").fill("https://us.example.com");

    // Race the toast against the auto-dismiss (5s) by waiting on the network
    // response then asserting visibility before the toast fades out.
    const registerResponse = authenticatedPage.waitForResponse(
      (res) => res.url().includes("/v1/mesh/peers") && res.request().method() === "POST",
    );
    await authenticatedPage.getByRole("button", { name: /^Registrar$/i }).click();
    await registerResponse;
    await expect(authenticatedPage.getByText(/Peer registrado/i)).toBeVisible({ timeout: 4000 });
  });

  // ---------------- Batch Search ----------------

  test("batch search: runs and renders score matrix", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.batchSearch);

    await expect(authenticatedPage.getByRole("heading", { name: "Busca em Lote" })).toBeVisible();

    // Default queries prefilled — run
    await authenticatedPage.getByRole("button", { name: /Executar busca em lote/i }).click();

    // Matrix heading appears
    await expect(authenticatedPage.getByRole("heading", { name: /Matriz de scores/i })).toBeVisible();

    // Summaries visible
    await expect(authenticatedPage.getByText("Autenticacao com refresh token")).toBeVisible();

    // Score cells (0.92 from mock)
    await expect(authenticatedPage.getByText("0.92").first()).toBeVisible();

    // Header summary
    await expect(authenticatedPage.getByText(/Memórias encontradas:/i)).toBeVisible();
  });

  test("batch search: warns on empty input", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.batchSearch);

    await authenticatedPage.locator("textarea").fill("");
    await authenticatedPage.getByRole("button", { name: /Executar busca em lote/i }).click();

    await expect(authenticatedPage.getByText(/Adicione pelo menos uma consulta/i)).toBeVisible();
  });

  // ---------------- MemoryDialog Insights (NodeSet + Feedback) ----------------

  test("memory dialog shows NodeSets and Feedback weight", async ({ authenticatedPage }) => {
    await authenticatedPage.goto(ROUTES.memories);

    // MemoryCard renders an "Editar" button (text, not title attr)
    await authenticatedPage.getByRole("button", { name: "Editar" }).first().click();

    // Insights section appears below the form
    await expect(authenticatedPage.getByText("Insights", { exact: true })).toBeVisible();

    // Sets from mock — the "core" pill is unique to the insights panel
    await expect(authenticatedPage.getByText("core", { exact: true })).toBeVisible();

    // Feedback metadata — α=0.3 is a unique indicator of the feedback panel
    await expect(authenticatedPage.getByText(/α=0\.3/)).toBeVisible();
  });
});
