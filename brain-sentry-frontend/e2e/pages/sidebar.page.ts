import { type Page, type Locator } from "@playwright/test";

export class Sidebar {
  readonly page: Page;
  readonly sidebar: Locator;
  readonly mobileMenuToggle: Locator;
  readonly mobileOverlay: Locator;
  readonly userEmail: Locator;

  private readonly navItems = [
    "Dashboard",
    "Memórias",
    "Busca",
    "Relacionamentos",
    "Console",
    "Traços de Agente",
    "Lab de Extração",
    "Ontologia",
    "Cache de Sessão",
    "Ações & Leases",
    "Sincronização Mesh",
    "Busca em Lote",
    "Linha do Tempo",
    "Grafo Global",
    "Ego-grafo",
    "Grafo Temporal",
    "Auditoria",
    "Usuários",
    "Tenants",
    "Configurações",
    "Diagnóstico",
    "Analytics",
    "Perfil",
    "Playground",
    "Conectores",
    "Notas",
    "Tarefas",
  ] as const;

  constructor(page: Page) {
    this.page = page;
    this.sidebar = page.locator("aside");
    // The hamburger button is the LAST button in the mobile top bar (after ThemeSelector)
    this.mobileMenuToggle = page.locator("div.md\\:hidden").first().locator("button").last();
    this.mobileOverlay = page.locator("div.md\\:hidden.fixed.inset-0.z-50");
    this.userEmail = page.locator("aside .text-xs.text-muted-foreground.truncate");
  }

  getNavItem(name: string): Locator {
    return this.sidebar.getByRole("button", { name, exact: true });
  }

  getMobileNavItem(name: string): Locator {
    return this.mobileOverlay.getByRole("button", { name, exact: true });
  }

  getActiveNavItem(): Locator {
    return this.sidebar.locator("nav button.bg-gradient-to-r");
  }

  getAllNavItems(): string[] {
    return [...this.navItems];
  }

  async navigateTo(name: string) {
    await this.getNavItem(name).click();
  }

  async openMobileMenu() {
    await this.mobileMenuToggle.click();
    await this.mobileOverlay.waitFor({ state: "visible" });
  }
}
