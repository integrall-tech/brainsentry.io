import { useState, useEffect, useCallback } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  Brain,
  Database,
  Search,
  TrendingUp,
  Activity,
  Clock,
  Tag,
  Plus,
  Network,
  Gauge,
  Zap,
  HelpCircle,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { MemoryCard } from "@/components/memory";
import { useToast } from "@/components/ui/toast";
import { useAuth } from "@/contexts/AuthContext";
import { api, resolveWsBaseUrl } from "@/lib/api";
import { KnowledgeGraph } from "@/components/visualizations/KnowledgeGraph";
import { ActivityHeatmap } from "@/components/visualizations/ActivityHeatmap";
import { LiveIndicator } from "@/components/visualizations/LiveIndicator";
import { useWebSocket } from "@/hooks/useWebSocket";
import type { WebSocketMessage } from "@/hooks/useWebSocket";

interface Stats {
  totalMemories: number;
  memoriesByCategory: Record<string, number>;
  memoriesByImportance: Record<string, number>;
  requestsToday: number;
  injectionRate: number;
  avgLatencyMs: number;
  helpfulnessRate: number;
  totalInjections: number;
  activeMemories24h: number;
}

interface RecentMemory {
  id: string;
  content: string;
  summary: string;
  category: string;
  importance: string;
  createdAt: string;
  tags: string[];
}

export function DashboardPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const navigate = useNavigate();
  const tenantId = user?.tenantId ?? "";

  const [stats, setStats] = useState<Stats | null>(null);
  const [memories, setMemories] = useState<RecentMemory[]>([]);
  const [statsLoading, setStatsLoading] = useState(true);
  const [memoriesLoading, setMemoriesLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [activeTab, setActiveTab] = useState<"overview" | "graph" | "activity">("overview");

  const fetchData = useCallback(async () => {
    setStatsLoading(true);
    setMemoriesLoading(true);
    setLoadError(false);
    try {
      const [statsData, memoriesData] = await Promise.all([
        api.getStats(),
        api.getMemories(0, 6),
      ]);
      setStats(statsData);
      setMemories(memoriesData.memories ?? []);
    } catch (error) {
      console.error("Error fetching dashboard data:", error);
      setLoadError(true);
    } finally {
      setStatsLoading(false);
      setMemoriesLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Realtime via WebSocket fica atrás de flag: o backend Go ainda não expõe
  // /ws (herança do legado Java). Sem a flag, nada de indicador "live"
  // mentindo reconexão eterna — um polling leve mantém os dados frescos.
  const wsEnabled = import.meta.env.VITE_WS_ENABLED === "true";
  const wsUrl = `${resolveWsBaseUrl()}/ws`;
  const handleWsMessage = useCallback((msg: WebSocketMessage) => {
    if (msg.type === "memory_created" || msg.type === "memory_updated" || msg.type === "memory_deleted") {
      fetchData();
      toast({
        title: t("dashboard.updated"),
        description: `Memory ${msg.type.replace("memory_", "")}`,
        variant: "info",
      });
    }
  }, [fetchData, toast, t]);

  const { status: wsStatus } = useWebSocket({
    url: wsUrl,
    onMessage: handleWsMessage,
    reconnect: true,
    enabled: wsEnabled,
  });

  useEffect(() => {
    if (wsEnabled) return;
    const timer = setInterval(fetchData, 60_000);
    return () => clearInterval(timer);
  }, [wsEnabled, fetchData]);

  const handleRefresh = () => {
    fetchData();
    toast({
      title: t("dashboard.updated"),
      description: t("dashboard.updatedDesc"),
      variant: "info",
    });
  };

  const categories = Object.entries(stats?.memoriesByCategory || {})
    .map(([category, count]) => ({ category, count }))
    .sort((a, b) => b.count - a.count);

  const importanceData = Object.entries(stats?.memoriesByImportance || {})
    .map(([level, count]) => ({ level, count }));

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-10 border-b bg-gradient-to-r from-brain-primary to-brain-accent text-white">
        <div className="px-4 py-[14px]">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-1.5 bg-white/20 rounded-lg backdrop-blur-sm">
                <Brain className="h-5 w-5 text-white" />
              </div>
              <div>
                <h1 className="text-base font-bold leading-tight">{t("dashboard.title")}</h1>
                <p className="text-xs text-white/80">
                  {t("dashboard.subtitle")}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              {wsEnabled && <LiveIndicator status={wsStatus} className="bg-white/10 px-2 py-1 rounded-full" />}
              <div className="text-xs text-white/80 hidden md:block">
                {t("dashboard.tenant")}: <span className="font-medium text-white">{tenantId ? `${tenantId.slice(0, 8)}...` : "—"}</span>
              </div>
              <Button variant="outline" size="sm" className="bg-white/20 border-white/30 text-white hover:bg-white/30" onClick={handleRefresh}>
                <Activity className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-6">
        {loadError && !statsLoading && (
          <div
            data-testid="dashboard-load-error"
            className="mb-6 flex items-center justify-between gap-3 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive"
            role="alert"
          >
            <span>{t("common.generic_error")}</span>
            <Button size="sm" variant="outline" onClick={fetchData}>
              {t("common.try_again")}
            </Button>
          </div>
        )}

        {/* Quick Actions + Tabs */}
        <div className="mb-6 flex items-center justify-between flex-wrap gap-3">
          <div className="flex gap-2">
            <Button size="sm" className="bg-gradient-to-r from-brain-primary to-brain-accent hover:from-brain-primary-dark hover:to-brain-accent-dark text-white" onClick={() => navigate("/app/memories")}>
              <Plus className="h-4 w-4 mr-2" />
              {t("dashboard.newMemory")}
            </Button>
            <Button size="sm" variant="outline" onClick={() => navigate("/app/search")}>
              <Search className="h-4 w-4 mr-2" />
              {t("dashboard.search")}
            </Button>
          </div>

          {/* View Tabs */}
          <div className="flex bg-muted rounded-lg p-0.5">
            {(["overview", "graph", "activity"] as const).map((tab) => (
              <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={`px-3 py-1.5 text-xs font-medium rounded-md transition-colors ${
                  activeTab === tab
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                }`}
              >
                {tab === "overview" && <Gauge className="h-3.5 w-3.5 inline mr-1" />}
                {tab === "graph" && <Network className="h-3.5 w-3.5 inline mr-1" />}
                {tab === "activity" && <Activity className="h-3.5 w-3.5 inline mr-1" />}
                {t(`dashboard.tabs.${tab}`)}
              </button>
            ))}
          </div>
        </div>

        {/* Stats Cards (always visible) */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-6">
          <StatsCard
            title={t("dashboard.totalMemories")}
            value={stats?.totalMemories || 0}
            icon={<Database className="h-5 w-5" />}
            loading={statsLoading}
          />
          <StatsCard
            title={t("dashboard.categories")}
            value={Object.keys(stats?.memoriesByCategory || {}).length}
            icon={<Tag className="h-5 w-5" />}
            loading={statsLoading}
          />
          <StatsCard
            title={t("dashboard.critical")}
            value={stats?.memoriesByImportance?.CRITICAL || 0}
            icon={<TrendingUp className="h-5 w-5" />}
            loading={statsLoading}
            accent={stats?.memoriesByImportance?.CRITICAL ? "destructive" : undefined}
          />
          <StatsCard
            title={t("dashboard.active24h")}
            value={stats?.activeMemories24h || 0}
            icon={<Clock className="h-5 w-5" />}
            loading={statsLoading}
          />
        </div>

        {/* System Metrics Row */}
        {stats && (stats.requestsToday > 0 || stats.totalInjections > 0) && (
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-6">
            <MetricCard label={t("dashboard.requestsToday")} value={stats.requestsToday} icon={<Zap className="h-4 w-4" />} />
            <MetricCard label={t("dashboard.injections")} value={stats.totalInjections} suffix={` (${(stats.injectionRate * 100).toFixed(1)}%)`} icon={<HelpCircle className="h-4 w-4" />} />
            <MetricCard label={t("dashboard.avgLatency")} value={`${stats.avgLatencyMs.toFixed(0)}ms`} icon={<Clock className="h-4 w-4" />} />
            <MetricCard label={t("dashboard.helpfulness")} value={`${(stats.helpfulnessRate * 100).toFixed(0)}%`} icon={<TrendingUp className="h-4 w-4" />} />
          </div>
        )}

        {/* Tab Content */}
        {activeTab === "overview" && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Recent Memories */}
            <div className="lg:col-span-2">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between">
                  <CardTitle>{t("dashboard.recentMemories")}</CardTitle>
                  <Button variant="ghost" size="sm" onClick={() => navigate("/app/memories")}>{t("dashboard.viewAll")}</Button>
                </CardHeader>
                <CardContent>
                  {memoriesLoading ? (
                    <div className="flex justify-center py-8">
                      <Spinner size="lg" />
                    </div>
                  ) : memories.length === 0 ? (
                    <div className="text-center py-8 text-muted-foreground">
                      <Brain className="h-12 w-12 mx-auto mb-4 opacity-50" />
                      <p>{t("dashboard.noMemories")}</p>
                      <p className="text-xs mt-1">{t("dashboard.createToSee")}</p>
                    </div>
                  ) : (
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      {memories.map((memory) => (
                        <MemoryCard key={memory.id} memory={memory} />
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>

            {/* Sidebar */}
            <div className="space-y-4">
              {/* Categories */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">{t("dashboard.byCategory")}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3">
                    {categories.map((cat) => (
                      <div key={cat.category}>
                        <div className="flex items-center justify-between mb-1">
                          <div className="flex items-center gap-2">
                            <div className="h-2 w-2 rounded-full" style={{ backgroundColor: getCategoryColor(cat.category) }} />
                            <span className="text-xs capitalize">{cat.category.toLowerCase()}</span>
                          </div>
                          <span className="text-xs font-medium">{cat.count}</span>
                        </div>
                        <div className="h-1.5 bg-muted rounded-full overflow-hidden">
                          <div
                            className="h-full rounded-full transition-all"
                            style={{
                              width: `${Math.min(100, (cat.count / (stats?.totalMemories || 1)) * 100)}%`,
                              backgroundColor: getCategoryColor(cat.category),
                            }}
                          />
                        </div>
                      </div>
                    ))}
                    {categories.length === 0 && (
                      <p className="text-xs text-muted-foreground text-center py-2">{t("dashboard.noData")}</p>
                    )}
                  </div>
                </CardContent>
              </Card>

              {/* Importance */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">{t("dashboard.byImportance")}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-3">
                    {importanceData.map(({ level, count }) => (
                      <div key={level} className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <div className="h-2 w-2 rounded-full" style={{ backgroundColor: getImportanceColor(level) }} />
                          <span className="text-xs capitalize">{level.toLowerCase()}</span>
                        </div>
                        <span className="text-xs font-bold" style={{ color: getImportanceColor(level) }}>{count}</span>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            </div>
          </div>
        )}

        {activeTab === "graph" && (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Network className="h-5 w-5" />
                {t("dashboard.knowledgeGraph")}
              </CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <KnowledgeGraph height="600px" limit={150} />
            </CardContent>
          </Card>
        )}

        {activeTab === "activity" && (
          <div className="space-y-6">
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Activity className="h-5 w-5" />
                  {t("dashboard.activityHeatmap")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <ActivityHeatmap />
              </CardContent>
            </Card>
          </div>
        )}
      </main>
    </div>
  );
}

interface StatsCardProps {
  title: string;
  value: number | string;
  icon?: React.ReactNode;
  loading?: boolean;
  suffix?: string;
  accent?: "destructive";
}

function StatsCard({ title, value, icon, loading, accent }: StatsCardProps) {
  return (
    <Card className={accent === "destructive" && Number(value) > 0 ? "border-destructive/50" : ""}>
      <CardContent className="p-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-xs text-muted-foreground">{title}</p>
            {loading ? (
              <Spinner size="sm" />
            ) : (
              <p className={`text-2xl font-bold ${accent === "destructive" && Number(value) > 0 ? "text-destructive" : ""}`}>
                {value}
              </p>
            )}
          </div>
          {icon && (
            <div className="p-2.5 bg-gradient-to-br from-brain-primary to-brain-accent rounded-lg text-white">
              {icon}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

function MetricCard({ label, value, suffix, icon }: { label: string; value: number | string; suffix?: string; icon: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 p-3 rounded-lg border bg-card">
      <div className="p-1.5 rounded-md bg-muted text-muted-foreground">{icon}</div>
      <div>
        <p className="text-[10px] text-muted-foreground uppercase tracking-wider">{label}</p>
        <p className="text-sm font-semibold">{value}{suffix}</p>
      </div>
    </div>
  );
}

function getCategoryColor(category: string): string {
  const colors: Record<string, string> = {
    INSIGHT: "#3b82f6", WARNING: "#ef4444", KNOWLEDGE: "#10b981",
    ACTION: "#f59e0b", CONTEXT: "#8b5cf6", REFERENCE: "#06b6d4",
    GENERAL: "#6b7280", DECISION: "#3b82f6", PATTERN: "#10b981",
    ANTIPATTERN: "#ef4444", DOMAIN: "#f59e0b", BUG: "#dc2626",
    OPTIMIZATION: "#8b5cf6", INTEGRATION: "#06b6d4",
  };
  return colors[category] || "#6b7280";
}

function getImportanceColor(level: string): string {
  const colors: Record<string, string> = {
    CRITICAL: "#ef4444", IMPORTANT: "#f59e0b", MINOR: "#10b981",
  };
  return colors[level] || "#6b7280";
}
