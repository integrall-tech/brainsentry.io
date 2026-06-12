import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import {
  Settings, Save, RotateCcw, Bell, Shield, Database, Sparkles, ChevronRight,
  Webhook, Plus, Trash2, Activity, Brain, ScanSearch, Loader2, CheckCircle, XCircle,
  AlertTriangle, CircleDot,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/filter";
import { Spinner } from "@/components/ui/spinner";
import { useToast } from "@/components/ui/toast";
import { useAuth } from "@/contexts/AuthContext";
import { api } from "@/lib/api";

import { API_BASE_URL as API_URL } from "@/lib/api/client";

interface ConfigSection {
  id: string;
  titleKey: string;
  descKey: string;
  icon: React.ElementType;
}

interface ConfigOption {
  key: string;
  labelKey: string;
  type: "text" | "number" | "boolean" | "select";
  value: string | number | boolean;
  options?: { value: string; label: string }[];
  descKey?: string;
}

const CONFIG_SECTIONS: ConfigSection[] = [
  { id: "general", titleKey: "configuration.sections.general", descKey: "configuration.sections.generalDesc", icon: Settings },
  { id: "notifications", titleKey: "configuration.sections.notifications", descKey: "configuration.sections.notificationsDesc", icon: Bell },
  { id: "security", titleKey: "configuration.sections.security", descKey: "configuration.sections.securityDesc", icon: Shield },
  { id: "embeddings", titleKey: "configuration.sections.embeddings", descKey: "configuration.sections.embeddingsDesc", icon: Sparkles },
  { id: "database", titleKey: "configuration.sections.database", descKey: "configuration.sections.databaseDesc", icon: Database },
  { id: "webhooks", titleKey: "configuration.sections.webhooks", descKey: "configuration.sections.webhooksDesc", icon: Webhook },
  { id: "circuit-breakers", titleKey: "configuration.sections.circuit", descKey: "configuration.sections.circuitDesc", icon: Activity },
  { id: "llm-metrics", titleKey: "configuration.sections.llm", descKey: "configuration.sections.llmDesc", icon: Brain },
  { id: "pii-scanner", titleKey: "configuration.sections.pii", descKey: "configuration.sections.piiDesc", icon: ScanSearch },
];

const DEFAULT_CONFIGS: Record<string, ConfigOption[]> = {
  general: [
    { key: "appName", labelKey: "configuration.fields.appName", type: "text", value: "Brain Sentry", descKey: "configuration.fields.appNameDesc" },
    { key: "sessionTimeout", labelKey: "configuration.fields.sessionTimeout", type: "number", value: 30, descKey: "configuration.fields.sessionTimeoutDesc" },
    { key: "defaultPageSize", labelKey: "configuration.fields.defaultPageSize", type: "select", value: 20, options: [
      { value: "10", label: "10" }, { value: "20", label: "20" }, { value: "50", label: "50" }, { value: "100", label: "100" },
    ]},
  ],
  notifications: [
    { key: "enabled", labelKey: "configuration.fields.enableNotifications", type: "boolean", value: true },
    { key: "emailAlerts", labelKey: "configuration.fields.emailAlerts", type: "boolean", value: false },
    { key: "errorNotifications", labelKey: "configuration.fields.notifyErrors", type: "boolean", value: true },
  ],
  security: [
    { key: "maxLoginAttempts", labelKey: "configuration.fields.maxLoginAttempts", type: "number", value: 5 },
    { key: "passwordMinLength", labelKey: "configuration.fields.passwordMinLength", type: "number", value: 8 },
    { key: "requireMFA", labelKey: "configuration.fields.requireMFA", type: "boolean", value: false },
  ],
  embeddings: [
    { key: "model", labelKey: "configuration.fields.embeddingModel", type: "select", value: "all-MiniLM-L6-v2", options: [
      { value: "all-MiniLM-L6-v2", label: "all-MiniLM-L6-v2 (Fast)" },
      { value: "all-mpnet-base-v2", label: "all-mpnet-base-v2 (Accurate)" },
    ]},
    { key: "dimension", labelKey: "configuration.fields.dimension", type: "number", value: 384, descKey: "configuration.fields.dimensionDesc" },
  ],
  database: [
    { key: "connectionTimeout", labelKey: "configuration.fields.connectionTimeout", type: "number", value: 30 },
    { key: "poolSize", labelKey: "configuration.fields.poolSize", type: "number", value: 10 },
    { key: "enableCache", labelKey: "configuration.fields.enableCache", type: "boolean", value: true },
  ],
};

interface CircuitBreakerState {
  name: string;
  state: string;
  failures: number;
  lastFailure?: string;
}

interface LLMMetric {
  model: string;
  totalRequests: number;
  totalTokens: number;
  totalCost: number;
  avgLatencyMs: number;
  errorRate: number;
}

interface PIIResult {
  found: boolean;
  entities: Array<{
    type: string;
    value: string;
    start: number;
    end: number;
  }>;
  maskedText: string;
}

export function ConfigurationPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const [activeSection, setActiveSection] = useState("general");
  const [configs, setConfigs] = useState<Record<string, ConfigOption[]>>(DEFAULT_CONFIGS);
  const [isSaving, setIsSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);

  const currentSection = CONFIG_SECTIONS.find((s) => s.id === activeSection);
  const currentOptions = configs[activeSection] || [];

  // Webhooks state
  interface WebhookItem { id: string; url: string; events: string[]; active: boolean; createdAt: string; }
  const [webhooks, setWebhooks] = useState<WebhookItem[]>([]);
  const [webhookUrl, setWebhookUrl] = useState("");
  const [webhookEvents, setWebhookEvents] = useState("memory.created,memory.updated");
  const [webhooksLoading, setWebhooksLoading] = useState(false);

  // Circuit breakers state
  const [circuitBreakers, setCircuitBreakers] = useState<CircuitBreakerState[]>([]);
  const [cbLoading, setCbLoading] = useState(false);

  // LLM metrics state
  const [llmMetrics, setLlmMetrics] = useState<LLMMetric[]>([]);
  const [llmLoading, setLlmLoading] = useState(false);

  // PII scanner state
  const [piiText, setPiiText] = useState("");
  const [piiResult, setPiiResult] = useState<PIIResult | null>(null);
  const [piiLoading, setPiiLoading] = useState(false);

  const handleValueChange = (key: string, value: string | number | boolean) => {
    setConfigs((prev) => ({
      ...prev,
      [activeSection]: prev[activeSection].map((opt) =>
        opt.key === key ? { ...opt, value } : opt
      ),
    }));
    setHasChanges(true);
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      // Save config to backend
      const configPayload: Record<string, any> = {};
      Object.entries(configs).forEach(([section, options]) => {
        options.forEach(opt => {
          configPayload[`${section}.${opt.key}`] = opt.value;
        });
      });
      await api.axiosInstance.put("/v1/config", configPayload);
      toast({ title: t("configuration.saved"), description: t("configuration.savedDesc"), variant: "success" });
      setHasChanges(false);
    } catch {
      // If the backend doesn't have a config endpoint yet, save locally
      localStorage.setItem("brain_sentry_config", JSON.stringify(configs));
      toast({ title: t("configuration.savedLocal"), description: t("configuration.savedLocalDesc"), variant: "success" });
      setHasChanges(false);
    } finally {
      setIsSaving(false);
    }
  };

  const handleReset = () => {
    setConfigs(DEFAULT_CONFIGS);
    setHasChanges(false);
    toast({ title: t("configuration.resetDone"), variant: "info" });
  };

  // Load saved configs from localStorage
  useEffect(() => {
    const saved = localStorage.getItem("brain_sentry_config");
    if (saved) {
      try { setConfigs(JSON.parse(saved)); } catch { /* ignore */ }
    }
  }, []);

  // Webhooks
  const fetchWebhooks = async () => {
    setWebhooksLoading(true);
    try {
      const resp = await api.axiosInstance.get<WebhookItem[]>("/v1/webhooks");
      setWebhooks(Array.isArray(resp.data) ? resp.data : []);
    } catch { setWebhooks([]); }
    finally { setWebhooksLoading(false); }
  };

  const createWebhook = async () => {
    if (!webhookUrl) return;
    try {
      await api.axiosInstance.post("/v1/webhooks", {
        url: webhookUrl, events: webhookEvents.split(",").map(e => e.trim()),
      });
      toast({ title: t("configuration.webhookCreated"), variant: "success" });
      setWebhookUrl("");
      fetchWebhooks();
    } catch (err: any) {
      toast({ title: t("common.error"), description: err.message, variant: "error" });
    }
  };

  const deleteWebhook = async (id: string) => {
    try {
      await api.axiosInstance.delete(`/v1/webhooks/${id}`);
      toast({ title: t("configuration.webhookRemoved"), variant: "success" });
      fetchWebhooks();
    } catch (err: any) {
      toast({ title: t("common.error"), description: err.message, variant: "error" });
    }
  };

  // Circuit Breakers
  const fetchCircuitBreakers = async () => {
    setCbLoading(true);
    try {
      const data = await api.getCircuitBreakers();
      setCircuitBreakers(Array.isArray(data) ? data : data?.breakers || []);
    } catch { setCircuitBreakers([]); }
    finally { setCbLoading(false); }
  };

  // LLM Metrics
  const fetchLlmMetrics = async () => {
    setLlmLoading(true);
    try {
      const data = await api.getLLMMetrics();
      setLlmMetrics(Array.isArray(data) ? data : data?.metrics || []);
    } catch { setLlmMetrics([]); }
    finally { setLlmLoading(false); }
  };

  // PII Scanner
  const handleScanPII = async () => {
    if (!piiText.trim()) return;
    setPiiLoading(true);
    setPiiResult(null);
    try {
      const data = await api.scanPII(piiText);
      setPiiResult(data);
    } catch (err: any) {
      toast({ title: t("configuration.piiScanError"), description: err.message, variant: "error" });
    } finally {
      setPiiLoading(false);
    }
  };

  // Load section data
  useEffect(() => {
    if (activeSection === "webhooks") fetchWebhooks();
    if (activeSection === "circuit-breakers") fetchCircuitBreakers();
    if (activeSection === "llm-metrics") fetchLlmMetrics();
  }, [activeSection]);

  const renderConfigInput = (option: ConfigOption) => {
    const baseId = `config-${activeSection}-${option.key}`;
    const label = t(option.labelKey);
    const description = option.descKey ? t(option.descKey) : undefined;
    switch (option.type) {
      case "boolean":
        return (
          <label className="flex items-center gap-3 cursor-pointer">
            <input id={baseId} type="checkbox" checked={option.value as boolean}
              onChange={(e) => handleValueChange(option.key, e.target.checked)}
              className="w-5 h-5 rounded border-gray-300 text-primary focus:ring-primary" />
            <span className="text-sm">{description || label}</span>
          </label>
        );
      case "select":
        return (
          <div className="space-y-1">
            <label htmlFor={baseId} className="text-sm font-medium">{label}</label>
            <select id={baseId} value={option.value as string}
              onChange={(e) => handleValueChange(option.key, e.target.value)}
              className="w-full h-9 rounded-md border border-input bg-background px-3 py-1">
              {option.options?.map((opt) => (<option key={opt.value} value={opt.value}>{opt.label}</option>))}
            </select>
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
          </div>
        );
      case "number":
        return (
          <div className="space-y-1">
            <label htmlFor={baseId} className="text-sm font-medium">{label}</label>
            <Input id={baseId} type="number" value={option.value as number}
              onChange={(e) => handleValueChange(option.key, Number(e.target.value))} className="w-full" />
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
          </div>
        );
      default:
        return (
          <div className="space-y-1">
            <label htmlFor={baseId} className="text-sm font-medium">{label}</label>
            <Input id={baseId} type="text" value={option.value as string}
              onChange={(e) => handleValueChange(option.key, e.target.value)} className="w-full" />
            {description && <p className="text-xs text-muted-foreground">{description}</p>}
          </div>
        );
    }
  };

  const cbStateIcon = (state: string) => {
    switch (state?.toLowerCase()) {
      case "closed": return <CheckCircle className="h-4 w-4 text-green-500" />;
      case "open": return <XCircle className="h-4 w-4 text-red-500" />;
      case "half-open": return <AlertTriangle className="h-4 w-4 text-yellow-500" />;
      default: return <CircleDot className="h-4 w-4 text-gray-400" />;
    }
  };

  return (
    <div className="min-h-screen bg-background">
      <header className="sticky top-0 z-10 border-b bg-gradient-to-r from-brain-primary to-brain-accent text-white">
        <div className="px-4 py-[14px]">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-1.5 bg-white/20 rounded-lg backdrop-blur-sm">
                <Settings className="h-5 w-5 text-white" />
              </div>
              <div>
                <h1 className="text-base font-bold leading-tight">{t("configuration.title")}</h1>
                <p className="text-xs text-white/80">{t("configuration.subtitle")}</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {hasChanges && (
                <Button variant="outline" className="bg-white/20 text-white border-white/30 hover:bg-white/30" onClick={handleReset} disabled={isSaving}>
                  <RotateCcw className="h-4 w-4 mr-2" /> {t("configuration.reset")}
                </Button>
              )}
              <Button className="bg-white text-brain-primary hover:bg-white/90" onClick={handleSave} disabled={!hasChanges || isSaving}>
                {isSaving ? <Spinner size="sm" /> : <Save className="h-4 w-4 mr-2" />}
                {t("configuration.saveChanges")}
              </Button>
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8">
        <div className="flex gap-6">
          <aside className="w-64 flex-shrink-0">
            <nav className="space-y-1">
              {CONFIG_SECTIONS.map((section) => {
                const Icon = section.icon;
                const isActive = activeSection === section.id;
                return (
                  <button key={section.id} onClick={() => setActiveSection(section.id)}
                    className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg text-left transition-colors ${
                      isActive ? "bg-gradient-to-r from-brain-primary to-brain-accent text-white shadow-md" : "hover:bg-accent"
                    }`}>
                    <Icon className="h-5 w-5" />
                    <div className="flex-1">
                      <p className="text-sm font-medium">{t(section.titleKey)}</p>
                      <p className={`text-xs ${isActive ? "text-white/70" : "text-muted-foreground"}`}>{t(section.descKey)}</p>
                    </div>
                    {isActive && <ChevronRight className="h-4 w-4" />}
                  </button>
                );
              })}
            </nav>
          </aside>

          <div className="flex-1">
            {/* Standard config sections */}
            {["general", "notifications", "security", "embeddings", "database"].includes(activeSection) && (
              <Card>
                <CardHeader>
                  <div className="flex items-center gap-3">
                    {currentSection && (
                      <div className="p-2 bg-gradient-to-br from-brain-primary to-brain-accent rounded-lg">
                        <currentSection.icon className="h-5 w-5 text-white" />
                      </div>
                    )}
                    <div>
                      <CardTitle>{currentSection ? t(currentSection.titleKey) : ""}</CardTitle>
                      <CardDescription>{currentSection ? t(currentSection.descKey) : ""}</CardDescription>
                    </div>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="space-y-6">
                    {currentOptions.map((option) => (
                      <div key={option.key} className="pb-4 border-b last:border-0">{renderConfigInput(option)}</div>
                    ))}
                  </div>
                </CardContent>
              </Card>
            )}

            {/* Webhooks */}
            {activeSection === "webhooks" && (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2"><Webhook className="h-5 w-5" /> {t("configuration.webhookRegistered")}</CardTitle>
                  <CardDescription>{t("configuration.webhookDesc")}</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex gap-2">
                    <Input placeholder={t("configuration.webhookPlaceholderUrl")} value={webhookUrl}
                      onChange={(e) => setWebhookUrl(e.target.value)} className="flex-1" />
                    <Input placeholder={t("configuration.webhookPlaceholderEvents")} value={webhookEvents}
                      onChange={(e) => setWebhookEvents(e.target.value)} className="flex-1" />
                    <Button onClick={createWebhook} disabled={!webhookUrl}>
                      <Plus className="h-4 w-4 mr-2" /> {t("configuration.webhookAdd")}
                    </Button>
                  </div>
                  {webhooksLoading ? (
                    <div className="flex justify-center py-4"><Spinner size="sm" /></div>
                  ) : webhooks.length === 0 ? (
                    <p className="text-sm text-muted-foreground text-center py-4">{t("configuration.webhookEmpty")}</p>
                  ) : (
                    <div className="space-y-2">
                      {webhooks.map((wh) => (
                        <div key={wh.id} className="flex items-center justify-between p-3 border rounded-md">
                          <div>
                            <p className="text-sm font-mono">{wh.url}</p>
                            <p className="text-xs text-muted-foreground">{wh.events?.join(", ") || "all"}</p>
                          </div>
                          <Button variant="ghost" size="icon" onClick={() => deleteWebhook(wh.id)}>
                            <Trash2 className="h-4 w-4 text-destructive" />
                          </Button>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            )}

            {/* Circuit Breakers */}
            {activeSection === "circuit-breakers" && (
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <div>
                      <CardTitle className="flex items-center gap-2"><Activity className="h-5 w-5" /> {t("configuration.cbTitle")}</CardTitle>
                      <CardDescription>{t("configuration.cbDesc")}</CardDescription>
                    </div>
                    <Button variant="outline" size="sm" onClick={fetchCircuitBreakers} disabled={cbLoading}>
                      <RotateCcw className="h-4 w-4" />
                    </Button>
                  </div>
                </CardHeader>
                <CardContent>
                  {cbLoading ? (
                    <div className="flex justify-center py-8"><Spinner size="sm" /></div>
                  ) : circuitBreakers.length === 0 ? (
                    <p className="text-sm text-muted-foreground text-center py-8">{t("configuration.cbEmpty")}</p>
                  ) : (
                    <div className="space-y-3">
                      {circuitBreakers.map((cb, idx) => (
                        <div key={idx} className="flex items-center justify-between p-4 border rounded-lg">
                          <div className="flex items-center gap-3">
                            {cbStateIcon(cb.state)}
                            <div>
                              <p className="text-sm font-medium">{cb.name}</p>
                              <p className="text-xs text-muted-foreground">{t("configuration.cbFailures", { count: cb.failures || 0 })}</p>
                            </div>
                          </div>
                          <span className={`px-3 py-1 rounded-full text-xs font-medium ${
                            cb.state?.toLowerCase() === "closed" ? "bg-green-100 text-green-700" :
                            cb.state?.toLowerCase() === "open" ? "bg-red-100 text-red-700" :
                            "bg-yellow-100 text-yellow-700"
                          }`}>
                            {cb.state || "UNKNOWN"}
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            )}

            {/* LLM Metrics */}
            {activeSection === "llm-metrics" && (
              <Card>
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <div>
                      <CardTitle className="flex items-center gap-2"><Brain className="h-5 w-5" /> {t("configuration.llmTitle")}</CardTitle>
                      <CardDescription>{t("configuration.llmDesc")}</CardDescription>
                    </div>
                    <Button variant="outline" size="sm" onClick={fetchLlmMetrics} disabled={llmLoading}>
                      <RotateCcw className="h-4 w-4" />
                    </Button>
                  </div>
                </CardHeader>
                <CardContent>
                  {llmLoading ? (
                    <div className="flex justify-center py-8"><Spinner size="sm" /></div>
                  ) : llmMetrics.length === 0 ? (
                    <p className="text-sm text-muted-foreground text-center py-8">{t("configuration.llmEmpty")}</p>
                  ) : (
                    <div className="space-y-4">
                      {llmMetrics.map((m, idx) => (
                        <div key={idx} className="p-4 border rounded-lg">
                          <div className="flex items-center justify-between mb-3">
                            <p className="font-medium">{m.model}</p>
                            <span className="text-xs bg-brain-primary/10 text-brain-primary px-2 py-1 rounded">
                              {t("configuration.requests", { count: m.totalRequests })}
                            </span>
                          </div>
                          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                            <div>
                              <p className="text-muted-foreground text-xs">{t("configuration.totalTokens")}</p>
                              <p className="font-medium">{(m.totalTokens || 0).toLocaleString()}</p>
                            </div>
                            <div>
                              <p className="text-muted-foreground text-xs">{t("configuration.totalCost")}</p>
                              <p className="font-medium">${(m.totalCost || 0).toFixed(4)}</p>
                            </div>
                            <div>
                              <p className="text-muted-foreground text-xs">{t("configuration.avgLatency")}</p>
                              <p className="font-medium">{(m.avgLatencyMs || 0).toFixed(0)}ms</p>
                            </div>
                            <div>
                              <p className="text-muted-foreground text-xs">{t("configuration.errorRate")}</p>
                              <p className={`font-medium ${(m.errorRate || 0) > 0.05 ? "text-red-600" : "text-green-600"}`}>
                                {((m.errorRate || 0) * 100).toFixed(1)}%
                              </p>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            )}

            {/* PII Scanner */}
            {activeSection === "pii-scanner" && (
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2"><ScanSearch className="h-5 w-5" /> {t("configuration.piiTitle")}</CardTitle>
                  <CardDescription>{t("configuration.piiDesc")}</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <textarea
                    value={piiText}
                    onChange={(e) => setPiiText(e.target.value)}
                    placeholder={t("configuration.piiPlaceholder")}
                    className="w-full h-32 rounded-md border border-input bg-background px-3 py-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-brain-primary/50"
                  />
                  <Button onClick={handleScanPII} disabled={piiLoading || !piiText.trim()}>
                    {piiLoading ? <Loader2 className="h-4 w-4 animate-spin mr-2" /> : <ScanSearch className="h-4 w-4 mr-2" />}
                    {t("configuration.piiScan")}
                  </Button>

                  {piiResult && (
                    <div className="space-y-4">
                      {piiResult.found ? (
                        <>
                          <div className="p-3 bg-red-50 border border-red-200 rounded-md">
                            <p className="text-sm font-medium text-red-700 flex items-center gap-2">
                              <AlertTriangle className="h-4 w-4" />
                              {t("configuration.piiFound", { count: piiResult.entities?.length || 0 })}
                            </p>
                          </div>
                          {piiResult.entities?.map((entity, idx) => (
                            <div key={idx} className="flex items-center gap-3 p-3 border rounded-md">
                              <span className="px-2 py-1 rounded bg-red-100 text-red-700 text-xs font-medium">{entity.type}</span>
                              <span className="text-sm font-mono">{entity.value}</span>
                              <span className="text-xs text-muted-foreground">{t("configuration.piiPosition", { start: entity.start, end: entity.end })}</span>
                            </div>
                          ))}
                          {piiResult.maskedText && (
                            <div>
                              <p className="text-sm font-medium mb-1">{t("configuration.piiMasked")}</p>
                              <pre className="text-sm bg-accent p-3 rounded-md whitespace-pre-wrap">{piiResult.maskedText}</pre>
                            </div>
                          )}
                        </>
                      ) : (
                        <div className="p-3 bg-green-50 border border-green-200 rounded-md">
                          <p className="text-sm font-medium text-green-700 flex items-center gap-2">
                            <CheckCircle className="h-4 w-4" />
                            {t("configuration.piiNotFound")}
                          </p>
                        </div>
                      )}
                    </div>
                  )}
                </CardContent>
              </Card>
            )}

            {/* Info Card */}
            <Card className="mt-6">
              <CardHeader><CardTitle className="text-base">{t("configuration.sysInfo")}</CardTitle></CardHeader>
              <CardContent>
                <dl className="grid grid-cols-2 gap-4 text-sm">
                  <div>
                    <dt className="text-muted-foreground">{t("configuration.version")}</dt>
                    <dd className="font-medium">1.0.0</dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">{t("configuration.environment")}</dt>
                    <dd className="font-medium">{import.meta.env.MODE === "production" ? t("configuration.envProd") : t("configuration.envDev")}</dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">{t("configuration.apiUrl")}</dt>
                    <dd className="font-mono text-xs">{API_URL}</dd>
                  </div>
                  <div>
                    <dt className="text-muted-foreground">{t("configuration.tenant")}</dt>
                    <dd className="font-medium">{user?.tenantId || "—"}</dd>
                  </div>
                </dl>
              </CardContent>
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}
