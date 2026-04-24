import { Outlet, useNavigate, useLocation } from "react-router-dom";
import {
  Menu,
  X,
  HelpCircle,
  FileText,
  Activity,
  LayoutDashboard,
  Search,
  Network,
  Shield,
  Settings,
  Users,
  Building2,
  User,
  Wand2,
  Plug,
  StickyNote,
  ListTodo,
  Clock,
  MessageSquare,
  Zap,
  FlaskConical,
  BookOpen,
  Database,
  CheckSquare,
  Share2,
  Layers3,
  Scale,
  ShieldCheck,
  CalendarClock,
  Brain,
  FileCode2,
  Globe2,
  GitBranch,
  Hourglass,
} from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { ThemeSelector } from "@/components/ui/theme-selector";
import { LanguageSwitcher } from "@/components/ui/language-switcher";
import { ScreenHelp } from "@/components/ui/ScreenHelp";
import { getHelpContent } from "@/lib/help/helpContent";
import { useAuth } from "@/contexts/AuthContext";

const VISITED_KEY = "brainsentry.help.visited";

function loadVisitedRoutes(): Set<string> {
  try {
    const raw = localStorage.getItem(VISITED_KEY);
    if (!raw) return new Set();
    const parsed = JSON.parse(raw);
    return new Set(Array.isArray(parsed) ? parsed : []);
  } catch {
    return new Set();
  }
}

function saveVisitedRoutes(routes: Set<string>) {
  try {
    localStorage.setItem(VISITED_KEY, JSON.stringify(Array.from(routes)));
  } catch {
    // storage unavailable — non-critical
  }
}

export function AdminLayout() {
  const { t, i18n } = useTranslation();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  const [visited, setVisited] = useState<Set<string>>(() => loadVisitedRoutes());
  const navigate = useNavigate();
  const location = useLocation();
  const { user } = useAuth();

  const helpRoute = (() => {
    const parts = location.pathname.split("/");
    while (parts.length > 2) {
      const candidate = parts.join("/");
      if (getHelpContent(candidate)) return candidate;
      parts.pop();
    }
    return location.pathname;
  })();

  const helpAvailable = Boolean(getHelpContent(helpRoute));
  const isNewScreen = helpAvailable && !visited.has(helpRoute);

  useEffect(() => {
    if (helpOpen && helpAvailable && !visited.has(helpRoute)) {
      const next = new Set(visited);
      next.add(helpRoute);
      setVisited(next);
      saveVisitedRoutes(next);
    }
  }, [helpOpen, helpAvailable, helpRoute, visited]);

  const navigation = [
    { title: t("nav.dashboard"), href: "/app/dashboard", icon: LayoutDashboard, id: "dashboard" },
    { title: t("nav.memories"), href: "/app/memories", icon: FileText, id: "memories" },
    { title: t("nav.search"), href: "/app/search", icon: Search, id: "search" },
    { title: t("nav.relationships"), href: "/app/relationships", icon: Network, id: "relationships" },
    { title: t("nav.console"), href: "/app/console", icon: MessageSquare, id: "console" },
    { title: t("nav.traces"), href: "/app/traces", icon: Zap, id: "traces" },
    { title: t("nav.extraction"), href: "/app/extraction", icon: FlaskConical, id: "extraction" },
    { title: t("nav.ontology"), href: "/app/ontology", icon: BookOpen, id: "ontology" },
    { title: t("nav.sessionCache"), href: "/app/session-cache", icon: Database, id: "session-cache" },
    { title: t("nav.actionsLeases"), href: "/app/actions", icon: CheckSquare, id: "actions" },
    { title: t("nav.meshSync"), href: "/app/mesh", icon: Share2, id: "mesh" },
    { title: t("nav.batchSearch"), href: "/app/batch-search", icon: Layers3, id: "batch-search" },
    { title: t("nav.timeline"), href: "/app/timeline", icon: Clock, id: "timeline" },
    { title: t("nav.decisions"), href: "/app/decisions", icon: Scale, id: "decisions" },
    { title: t("nav.policies"), href: "/app/policies", icon: ShieldCheck, id: "policies" },
    { title: t("nav.events"), href: "/app/events", icon: CalendarClock, id: "events" },
    { title: t("nav.reasoning"), href: "/app/reasoning", icon: Brain, id: "reasoning" },
    { title: t("nav.provenance"), href: "/app/provenance", icon: FileCode2, id: "provenance" },
    { title: t("nav.graphGlobal"), href: "/app/graph/global", icon: Globe2, id: "graph-global" },
    { title: t("nav.graphEgo"), href: "/app/graph/ego", icon: GitBranch, id: "graph-ego" },
    { title: t("nav.graphTimeline"), href: "/app/graph/timeline", icon: Hourglass, id: "graph-timeline" },
    { title: t("nav.audit"), href: "/app/audit", icon: Shield, id: "audit" },
    { title: t("nav.users"), href: "/app/users", icon: Users, id: "users" },
    { title: t("nav.tenants"), href: "/app/tenants", icon: Building2, id: "tenants" },
    { title: t("nav.configuration"), href: "/app/configuration", icon: Settings, id: "configuration" },
    { title: t("nav.analytics"), href: "/app/analytics", icon: Activity, id: "analytics" },
    { title: t("nav.profile"), href: "/app/profile", icon: User, id: "profile" },
    { title: t("nav.playground"), href: "/app/playground", icon: Wand2, id: "playground" },
    { title: t("nav.connectors"), href: "/app/connectors", icon: Plug, id: "connectors" },
    { title: t("nav.notes"), href: "/app/notes", icon: StickyNote, id: "notes" },
    { title: t("nav.tasks"), href: "/app/tasks", icon: ListTodo, id: "tasks" },
  ];

  const handleNavigation = (href: string) => {
    navigate(href);
  };

  const activePath = navigation.find((item) =>
    location.pathname.startsWith(item.href)
  )?.id || "dashboard";

  return (
    <div className="h-screen flex flex-col md:flex-row overflow-hidden">
      {/* Top bar for mobile */}
      <div className="md:hidden flex items-center justify-between p-4 border-b bg-background">
        <div className="flex items-center gap-2">
          <img src="/images/brainsentry_logo.png" alt="Brain Sentry" className="h-8 w-auto" />
          <h1 className="text-xl font-bold">Brain Sentry</h1>
        </div>
        <div className="flex items-center gap-2">
          <ThemeSelector />
          <button
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
            className="p-2 rounded-md hover:bg-muted"
          >
            {mobileMenuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
        </div>
      </div>

      {/* Sidebar for desktop */}
      <aside className="hidden md:flex w-64 flex-col border-r bg-muted/40 h-screen">
        <div className="px-4 py-[14px] border-b bg-gradient-to-r from-brain-primary/10 to-brain-accent/10">
          <div className="flex items-center gap-2">
            <img src="/images/brainsentry_logo.png" alt="Brain Sentry" className="h-8 w-auto" />
            <div>
              <h2 className="text-base font-bold leading-tight">Brain Sentry</h2>
              <p className="text-xs text-muted-foreground">Admin Console</p>
            </div>
          </div>
        </div>
        <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
          {navigation.map((item) => {
            const isActive = activePath === item.id;
            return (
              <button
                key={item.id}
                onClick={() => handleNavigation(item.href)}
                className={cn(
                  "w-full flex items-center gap-3 px-4 py-2 rounded-md text-sm font-medium transition-all",
                  isActive
                    ? "bg-gradient-to-r from-brain-primary to-brain-accent text-white shadow-md"
                    : "text-muted-foreground hover:bg-muted/50 hover:text-accent-foreground"
                )}
              >
                <item.icon className="h-4 w-4" />
                {item.title}
              </button>
            );
          })}
        </nav>
        <div className="p-4 border-t">
          <div className="flex items-center justify-between">
            <div className="text-xs text-muted-foreground">
              v1.0.0
            </div>
            <div className="flex items-center gap-1">
              <LanguageSwitcher />
              <ThemeSelector />
            </div>
          </div>
          <div className="text-xs text-muted-foreground mt-1">
            {new Date().toLocaleDateString(i18n.language)}
          </div>
          {user && (
            <div className="text-xs text-muted-foreground mt-1 truncate">
              {user.email}
            </div>
          )}
        </div>
      </aside>

      {/* Mobile menu */}
      {mobileMenuOpen && (
        <div className="md:hidden fixed inset-0 z-50 bg-background">
        <div className="flex flex-col h-full">
            <div className="flex items-center justify-between p-4 border-b">
              <h2 className="text-lg font-bold">{t("nav.menu", "Menu")}</h2>
              <button
                onClick={() => setMobileMenuOpen(false)}
                className="p-2 rounded-md hover:bg-muted"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
            <nav className="flex-1 p-4 space-y-1 overflow-y-auto">
              {navigation.map((item) => (
                <button
                  key={item.id}
                  onClick={() => {
                    handleNavigation(item.href);
                    setMobileMenuOpen(false);
                  }}
                  className={cn(
                    "w-full flex items-center gap-3 px-4 py-3 rounded-md text-sm font-medium transition-all",
                    activePath === item.id
                      ? "bg-gradient-to-r from-brain-primary to-brain-accent text-white"
                      : "text-muted-foreground hover:bg-muted/50"
                  )}
                >
                  <item.icon className="h-4 w-4" />
                  {item.title}
                </button>
              ))}
            </nav>
          </div>
        </div>
      )}

      {/* Main content */}
      <main className="flex-1 overflow-y-auto min-h-0 relative">
        <Outlet />

        {/* Floating help button — reads current route and opens ScreenHelp */}
        {helpAvailable && (
          <button
            type="button"
            data-testid="screen-help-trigger"
            aria-label={t("help.button.aria")}
            onClick={() => setHelpOpen(true)}
            className={cn(
              "fixed bottom-5 left-5 md:left-[17rem] z-40 flex items-center gap-2 rounded-full shadow-lg",
              "px-3.5 py-2.5 text-xs font-medium transition-all",
              "bg-gradient-to-r from-brain-primary to-brain-accent text-white",
              "hover:shadow-xl hover:scale-[1.03]",
            )}
          >
            <HelpCircle className="h-4 w-4" />
            <span className="hidden sm:inline">{t("help.button.label")}</span>
            {isNewScreen && (
              <span
                data-testid="screen-help-new-badge"
                className="absolute -top-1 -right-1 h-3 w-3 rounded-full bg-red-500 border-2 border-background"
              />
            )}
          </button>
        )}
      </main>

      {helpAvailable && (
        <ScreenHelp
          open={helpOpen}
          onClose={() => setHelpOpen(false)}
          route={helpRoute}
        />
      )}
    </div>
  );
}
