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
  ChevronDown,
  ChevronRight,
  Gavel,
  Sparkles,
  Workflow,
  Lightbulb,
  Link2,
  BarChart3,
  SlidersHorizontal,
  type LucideIcon,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";
import { ThemeSelector } from "@/components/ui/theme-selector";
import { LanguageSwitcher } from "@/components/ui/language-switcher";
import { ScreenHelp } from "@/components/ui/ScreenHelp";
import { getHelpContent } from "@/lib/help/helpContent";
import { useAuth } from "@/contexts/AuthContext";

const VISITED_KEY = "brainsentry.help.visited";
const NAV_EXPANDED_KEY = "brainsentry.nav.expanded";

type NavLink = {
  kind: "item";
  id: string;
  title: string;
  href: string;
  icon: LucideIcon;
};

type NavGroup = {
  kind: "group";
  id: string;
  title: string;
  icon: LucideIcon;
  items: NavLink[];
};

type NavEntry = NavLink | NavGroup;

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

const ALL_GROUP_IDS = [
  "knowledge",
  "graphs",
  "decisions",
  "intelligence",
  "coordination",
  "analysis",
  "admin",
];

function loadExpandedGroups(): Set<string> {
  try {
    const raw = localStorage.getItem(NAV_EXPANDED_KEY);
    if (raw === null) return new Set(ALL_GROUP_IDS);
    const parsed = JSON.parse(raw);
    return new Set(Array.isArray(parsed) ? parsed : ALL_GROUP_IDS);
  } catch {
    return new Set(ALL_GROUP_IDS);
  }
}

function saveExpandedGroups(ids: Set<string>) {
  try {
    localStorage.setItem(NAV_EXPANDED_KEY, JSON.stringify(Array.from(ids)));
  } catch {
    // storage unavailable — non-critical
  }
}

export function AdminLayout() {
  const { t, i18n } = useTranslation();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  const [visited, setVisited] = useState<Set<string>>(() => loadVisitedRoutes());
  const [expanded, setExpanded] = useState<Set<string>>(() => loadExpandedGroups());
  const navigate = useNavigate();
  const location = useLocation();
  const { user } = useAuth();

  const navigation = useMemo<NavEntry[]>(
    () => [
      { kind: "item", id: "dashboard", title: t("nav.dashboard"), href: "/app/dashboard", icon: LayoutDashboard },
      {
        kind: "group",
        id: "knowledge",
        title: t("nav.group.knowledge"),
        icon: Brain,
        items: [
          { kind: "item", id: "memories", title: t("nav.memories"), href: "/app/memories", icon: FileText },
          { kind: "item", id: "search", title: t("nav.search"), href: "/app/search", icon: Search },
          { kind: "item", id: "batch-search", title: t("nav.batchSearch"), href: "/app/batch-search", icon: Layers3 },
          { kind: "item", id: "relationships", title: t("nav.relationships"), href: "/app/relationships", icon: Link2 },
          { kind: "item", id: "timeline", title: t("nav.timeline"), href: "/app/timeline", icon: Clock },
          { kind: "item", id: "notes", title: t("nav.notes"), href: "/app/notes", icon: StickyNote },
        ],
      },
      {
        kind: "group",
        id: "graphs",
        title: t("nav.group.graphs"),
        icon: Network,
        items: [
          { kind: "item", id: "graph-global", title: t("nav.graphGlobal"), href: "/app/graph/global", icon: Globe2 },
          { kind: "item", id: "graph-ego", title: t("nav.graphEgo"), href: "/app/graph/ego", icon: GitBranch },
          { kind: "item", id: "graph-timeline", title: t("nav.graphTimeline"), href: "/app/graph/timeline", icon: Hourglass },
        ],
      },
      {
        kind: "group",
        id: "decisions",
        title: t("nav.group.decisions"),
        icon: Gavel,
        items: [
          { kind: "item", id: "decisions", title: t("nav.decisions"), href: "/app/decisions", icon: Scale },
          { kind: "item", id: "policies", title: t("nav.policies"), href: "/app/policies", icon: ShieldCheck },
          { kind: "item", id: "events", title: t("nav.events"), href: "/app/events", icon: CalendarClock },
          { kind: "item", id: "reasoning", title: t("nav.reasoning"), href: "/app/reasoning", icon: Lightbulb },
          { kind: "item", id: "provenance", title: t("nav.provenance"), href: "/app/provenance", icon: FileCode2 },
        ],
      },
      {
        kind: "group",
        id: "intelligence",
        title: t("nav.group.intelligence"),
        icon: Sparkles,
        items: [
          { kind: "item", id: "console", title: t("nav.console"), href: "/app/console", icon: MessageSquare },
          { kind: "item", id: "extraction", title: t("nav.extraction"), href: "/app/extraction", icon: FlaskConical },
          { kind: "item", id: "ontology", title: t("nav.ontology"), href: "/app/ontology", icon: BookOpen },
          { kind: "item", id: "session-cache", title: t("nav.sessionCache"), href: "/app/session-cache", icon: Database },
          { kind: "item", id: "traces", title: t("nav.traces"), href: "/app/traces", icon: Zap },
        ],
      },
      {
        kind: "group",
        id: "coordination",
        title: t("nav.group.coordination"),
        icon: Workflow,
        items: [
          { kind: "item", id: "actions", title: t("nav.actionsLeases"), href: "/app/actions", icon: CheckSquare },
          { kind: "item", id: "mesh", title: t("nav.meshSync"), href: "/app/mesh", icon: Share2 },
          { kind: "item", id: "connectors", title: t("nav.connectors"), href: "/app/connectors", icon: Plug },
          { kind: "item", id: "tasks", title: t("nav.tasks"), href: "/app/tasks", icon: ListTodo },
        ],
      },
      {
        kind: "group",
        id: "analysis",
        title: t("nav.group.analysis"),
        icon: Activity,
        items: [
          { kind: "item", id: "analytics", title: t("nav.analytics"), href: "/app/analytics", icon: BarChart3 },
          { kind: "item", id: "audit", title: t("nav.audit"), href: "/app/audit", icon: Shield },
          { kind: "item", id: "playground", title: t("nav.playground"), href: "/app/playground", icon: Wand2 },
        ],
      },
      {
        kind: "group",
        id: "admin",
        title: t("nav.group.admin"),
        icon: Settings,
        items: [
          { kind: "item", id: "users", title: t("nav.users"), href: "/app/users", icon: Users },
          { kind: "item", id: "tenants", title: t("nav.tenants"), href: "/app/tenants", icon: Building2 },
          { kind: "item", id: "configuration", title: t("nav.configuration"), href: "/app/configuration", icon: SlidersHorizontal },
          { kind: "item", id: "profile", title: t("nav.profile"), href: "/app/profile", icon: User },
        ],
      },
    ],
    [t],
  );

  // Flatten for active-path lookup and route → parent-group lookup
  const flatItems = useMemo<NavLink[]>(() => {
    const out: NavLink[] = [];
    for (const e of navigation) {
      if (e.kind === "item") out.push(e);
      else out.push(...e.items);
    }
    return out;
  }, [navigation]);

  const groupOf = useMemo<Map<string, string>>(() => {
    const m = new Map<string, string>();
    for (const e of navigation) {
      if (e.kind === "group") {
        for (const it of e.items) m.set(it.id, e.id);
      }
    }
    return m;
  }, [navigation]);

  const activeItemId =
    flatItems
      .filter((it) => location.pathname.startsWith(it.href))
      .sort((a, b) => b.href.length - a.href.length)[0]?.id ?? "dashboard";

  const activeGroupId = groupOf.get(activeItemId);

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

  const isExpanded = (groupId: string) => expanded.has(groupId) || activeGroupId === groupId;

  const toggleGroup = (groupId: string) => {
    const next = new Set(expanded);
    if (next.has(groupId)) next.delete(groupId);
    else next.add(groupId);
    setExpanded(next);
    saveExpandedGroups(next);
  };

  const handleNavigation = (href: string) => {
    navigate(href);
  };

  const renderNavTree = (onItemClick?: () => void, isMobile = false) => (
    <>
      {navigation.map((entry) => {
        if (entry.kind === "item") {
          const isActive = activeItemId === entry.id;
          return (
            <button
              key={entry.id}
              onClick={() => {
                handleNavigation(entry.href);
                onItemClick?.();
              }}
              className={cn(
                "w-full flex items-center gap-3 px-4 rounded-md text-sm font-medium transition-all",
                isMobile ? "py-3" : "py-2",
                isActive
                  ? "bg-gradient-to-r from-brain-primary to-brain-accent text-white shadow-md"
                  : "text-muted-foreground hover:bg-muted/50 hover:text-accent-foreground",
              )}
            >
              <entry.icon className="h-4 w-4" />
              {entry.title}
            </button>
          );
        }

        const open = isExpanded(entry.id);
        const groupActive = activeGroupId === entry.id;
        return (
          <div key={entry.id} className="space-y-0.5">
            <button
              type="button"
              data-testid={`nav-group-${entry.id}`}
              aria-expanded={open}
              onClick={() => toggleGroup(entry.id)}
              className={cn(
                "w-full flex items-center gap-2 px-3 py-1.5 rounded-md text-[11px] font-semibold uppercase tracking-wider transition-colors",
                groupActive
                  ? "text-foreground"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              <entry.icon className="h-3.5 w-3.5 flex-shrink-0" />
              <span className="flex-1 text-left">{entry.title}</span>
              {open ? (
                <ChevronDown className="h-3.5 w-3.5" />
              ) : (
                <ChevronRight className="h-3.5 w-3.5" />
              )}
            </button>
            {open && (
              <div className="space-y-0.5 pl-3 border-l border-border/40 ml-4">
                {entry.items.map((item) => {
                  const isActive = activeItemId === item.id;
                  return (
                    <button
                      key={item.id}
                      onClick={() => {
                        handleNavigation(item.href);
                        onItemClick?.();
                      }}
                      className={cn(
                        "w-full flex items-center gap-3 px-3 rounded-md text-sm font-medium transition-all",
                        isMobile ? "py-2.5" : "py-1.5",
                        isActive
                          ? "bg-gradient-to-r from-brain-primary to-brain-accent text-white shadow-sm"
                          : "text-muted-foreground hover:bg-muted/50 hover:text-accent-foreground",
                      )}
                    >
                      <item.icon className="h-3.5 w-3.5" />
                      {item.title}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
    </>
  );

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
        <nav className="flex-1 p-3 space-y-1.5 overflow-y-auto">
          {renderNavTree()}
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
            <nav className="flex-1 p-3 space-y-1.5 overflow-y-auto">
              {renderNavTree(() => setMobileMenuOpen(false), true)}
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
