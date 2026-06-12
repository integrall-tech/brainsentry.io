import { useState, useEffect, useCallback, useRef } from "react";
import { useTranslation } from "react-i18next";
import {
  Network, Search, ArrowRight, Trash2, RefreshCw, Users, Package, Building,
  MapPin, Calendar, Tag, CircleDot, Link2, Zap, MessageSquare, Loader2,
  Sparkles, Brain,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui";
import { Button } from "@/components/ui/button";
import { SearchInput } from "@/components/ui/filter";
import { Spinner, Skeleton } from "@/components/ui/spinner";
import { CategoryTag } from "@/components/ui/tags";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useDebounce } from "@/hooks";
import { useToast } from "@/components/ui/toast";
import { useAuth } from "@/contexts/AuthContext";
import { api, API_BASE_URL as API_URL } from "@/lib/api/client";
import CytoscapeComponent from "react-cytoscapejs";

interface Memory {
  id: string;
  content: string;
  summary: string;
  category: string;
  importance: string;
  tags: string[];
}

interface Relationship {
  id: string;
  fromMemoryId: string;
  toMemoryId: string;
  type: string;
  strength: number;
  createdAt: string;
  fromMemory?: Memory;
  toMemory?: Memory;
}

interface EntityNode {
  id: string;
  name: string;
  type: string;
  sourceMemoryId?: string;
  properties?: Record<string, string>;
}

interface EntityEdge {
  id: string;
  sourceId: string;
  targetId: string;
  sourceName: string;
  targetName: string;
  type: string;
  properties?: Record<string, string>;
}

interface KnowledgeGraphResponse {
  nodes: EntityNode[];
  edges: EntityEdge[];
  totalNodes: number;
  totalEdges: number;
}

interface Community {
  id: number;
  members: string[];
  label?: string;
  size: number;
}

const RELATIONSHIP_TYPE_KEYS = ["RELATED", "DEPENDS_ON", "DEPENDENT_OF", "CONTRADICTS", "SIMILAR_TO", "EXTENDS"] as const;

const ENTITY_TYPE_CONFIG: Record<string, { color: string; bgColor: string; icon: typeof Users; cyColor: string }> = {
  PESSOA: { color: "text-blue-600", bgColor: "bg-blue-100", icon: Users, cyColor: "#3B82F6" },
  CLIENTE: { color: "text-blue-600", bgColor: "bg-blue-100", icon: Users, cyColor: "#3B82F6" },
  VENDEDOR: { color: "text-green-600", bgColor: "bg-green-100", icon: Users, cyColor: "#22C55E" },
  ORGANIZACAO: { color: "text-purple-600", bgColor: "bg-purple-100", icon: Building, cyColor: "#A855F7" },
  EMPRESA: { color: "text-purple-600", bgColor: "bg-purple-100", icon: Building, cyColor: "#A855F7" },
  PRODUTO: { color: "text-orange-600", bgColor: "bg-orange-100", icon: Package, cyColor: "#F97316" },
  PEDIDO: { color: "text-red-600", bgColor: "bg-red-100", icon: Tag, cyColor: "#EF4444" },
  LOCAL: { color: "text-cyan-600", bgColor: "bg-cyan-100", icon: MapPin, cyColor: "#06B6D4" },
  ENDERECO: { color: "text-cyan-600", bgColor: "bg-cyan-100", icon: MapPin, cyColor: "#06B6D4" },
  DATA: { color: "text-yellow-600", bgColor: "bg-yellow-100", icon: Calendar, cyColor: "#EAB308" },
  EVENTO: { color: "text-pink-600", bgColor: "bg-pink-100", icon: Calendar, cyColor: "#EC4899" },
  CONCEITO: { color: "text-gray-600", bgColor: "bg-gray-100", icon: CircleDot, cyColor: "#6B7280" },
};

const COMMUNITY_COLORS = ["#3B82F6", "#22C55E", "#A855F7", "#F97316", "#EF4444", "#06B6D4", "#EAB308", "#EC4899", "#6B7280", "#14B8A6"];

function getEntityConfig(type: string) {
  return ENTITY_TYPE_CONFIG[type?.toUpperCase()] || {
    color: "text-gray-600", bgColor: "bg-gray-100", icon: CircleDot, cyColor: "#6B7280",
  };
}

export function RelationshipsPage() {
  const { t } = useTranslation();
  const { user } = useAuth();
  const { toast } = useToast();
  const cyRef = useRef<any>(null);
  const RELATIONSHIP_TYPES = RELATIONSHIP_TYPE_KEYS.map((k) => ({
    value: k,
    label: t(`relationships.types.${k}`),
  }));

  // Tab state
  const [activeTab, setActiveTab] = useState<"graph" | "knowledge" | "memory">("graph");

  // State
  const [searchQuery, setSearchQuery] = useState("");
  const debouncedSearchQuery = useDebounce(searchQuery, 500);
  const [selectedMemory, setSelectedMemory] = useState<Memory | null>(null);
  const [relationshipType, setRelationshipType] = useState("RELATED");
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);

  // Memory relationships state
  const [relationships, setRelationships] = useState<Relationship[]>([]);
  const [totalElements, setTotalElements] = useState(0);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  // Knowledge graph state
  const [knowledgeGraph, setKnowledgeGraph] = useState<KnowledgeGraphResponse | null>(null);
  const [isLoadingGraph, setIsLoadingGraph] = useState(false);

  // Communities state
  const [communities, setCommunities] = useState<Community[]>([]);
  const [communitiesLoading, setCommunitiesLoading] = useState(false);

  // NL Query state
  const [nlQuestion, setNlQuestion] = useState("");
  const [nlAnswer, setNlAnswer] = useState<any>(null);
  const [nlLoading, setNlLoading] = useState(false);

  // Activation state
  const [activationSeedId, setActivationSeedId] = useState("");
  const [activationResults, setActivationResults] = useState<any>(null);
  const [activationLoading, setActivationLoading] = useState(false);

  // Search state
  const [searchResults, setSearchResults] = useState<Memory[]>([]);
  const [isSearching, setIsSearching] = useState(false);

  // Highlighted memory state
  const [highlightedMemory, setHighlightedMemory] = useState<Memory | null>(null);
  const [highlightedConnections, setHighlightedConnections] = useState<Relationship[]>([]);
  const [isLoadingConnections, setIsLoadingConnections] = useState(false);

  // Dialog state for target memory selection
  const [targetMemory, setTargetMemory] = useState<Memory | null>(null);
  const [dialogSearchQuery, setDialogSearchQuery] = useState("");
  const [dialogSearchResults, setDialogSearchResults] = useState<Memory[]>([]);
  const [isDialogSearching, setIsDialogSearching] = useState(false);
  const debouncedDialogSearch = useDebounce(dialogSearchQuery, 500);

  // Reprocess state
  const [isReprocessing, setIsReprocessing] = useState(false);

  // Fetch knowledge graph
  const fetchKnowledgeGraph = useCallback(async () => {
    setIsLoadingGraph(true);
    try {
      const response = await api.axiosInstance.get<KnowledgeGraphResponse>(
        `/v1/entity-graph/knowledge-graph?limit=100`
      );
      setKnowledgeGraph(response.data);
    } catch (err) {
      console.error("Error fetching knowledge graph:", err);
      setKnowledgeGraph(null);
    } finally {
      setIsLoadingGraph(false);
    }
  }, []);

  // Fetch memory relationships
  const fetchRelationships = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await api.axiosInstance.get<{ relationships: Relationship[]; totalElements: number }>(
        `/v1/relationships?page=${page - 1}&size=${pageSize}`
      );
      setRelationships(response.data.relationships || []);
      setTotalElements(response.data.totalElements || 0);
    } catch (err) {
      setError(err as Error);
    } finally {
      setIsLoading(false);
    }
  }, [page, pageSize]);

  // Fetch communities
  const fetchCommunities = useCallback(async () => {
    setCommunitiesLoading(true);
    try {
      const data = await api.getCommunities();
      setCommunities(Array.isArray(data) ? data : data?.communities || []);
    } catch (err) {
      console.error("Error fetching communities:", err);
      setCommunities([]);
    } finally {
      setCommunitiesLoading(false);
    }
  }, []);

  // Search memories
  const shouldSearch = debouncedSearchQuery && debouncedSearchQuery.length >= 2;

  const searchMemories = useCallback(async () => {
    if (!shouldSearch) { setSearchResults([]); return; }
    setIsSearching(true);
    try {
      const response = await api.axiosInstance.post<Memory[]>(
        `/v1/memories/search`, { query: debouncedSearchQuery, limit: 10 }
      );
      setSearchResults(response.data || []);
    } catch (err) {
      console.error("Search error:", err);
      setSearchResults([]);
    } finally {
      setIsSearching(false);
    }
  }, [shouldSearch, debouncedSearchQuery]);

  // Fetch connections for a specific memory
  const fetchConnectionsForMemory = useCallback(async (memoryId: string) => {
    setIsLoadingConnections(true);
    try {
      const response = await api.axiosInstance.get<{ relationships: Relationship[]; totalElements: number }>(
        `/v1/relationships?page=0&size=100`
      );
      const allRelationships = response.data.relationships || [];
      const filtered = allRelationships.filter(
        r => r.fromMemoryId === memoryId || r.toMemoryId === memoryId
      );
      setHighlightedConnections(filtered);
    } catch (err) {
      console.error("Error fetching connections:", err);
      setHighlightedConnections([]);
    } finally {
      setIsLoadingConnections(false);
    }
  }, []);

  useEffect(() => {
    fetchKnowledgeGraph();
    fetchRelationships();
    fetchCommunities();
  }, [fetchKnowledgeGraph, fetchRelationships, fetchCommunities]);

  useEffect(() => { searchMemories(); }, [searchMemories]);

  useEffect(() => {
    const searchInDialog = async () => {
      if (!debouncedDialogSearch || debouncedDialogSearch.length < 2) { setDialogSearchResults([]); return; }
      setIsDialogSearching(true);
      try {
        const response = await api.axiosInstance.post<Memory[]>(
          `/v1/memories/search`, { query: debouncedDialogSearch, limit: 10 }
        );
        setDialogSearchResults((response.data || []).filter(m => m.id !== selectedMemory?.id));
      } catch (err) { setDialogSearchResults([]); }
      finally { setIsDialogSearching(false); }
    };
    searchInDialog();
  }, [debouncedDialogSearch, selectedMemory?.id]);

  useEffect(() => {
    if (!showCreateDialog) { setTargetMemory(null); setDialogSearchQuery(""); setDialogSearchResults([]); }
  }, [showCreateDialog]);

  const refetch = () => { fetchKnowledgeGraph(); fetchRelationships(); fetchCommunities(); };

  // NL Query handler
  const handleNlQuery = async () => {
    if (!nlQuestion.trim()) return;
    setNlLoading(true);
    setNlAnswer(null);
    try {
      const data = await api.nlQuery(nlQuestion);
      setNlAnswer(data);
    } catch (err) {
      toast({ title: t("relationships.askError"), description: (err as Error).message, variant: "error" });
    } finally {
      setNlLoading(false);
    }
  };

  // Spreading Activation handler
  const handleActivation = async () => {
    if (!activationSeedId.trim()) return;
    setActivationLoading(true);
    setActivationResults(null);
    try {
      const data = await api.activateMemories([activationSeedId]);
      setActivationResults(data);
    } catch (err) {
      toast({ title: t("relationships.activationError"), description: (err as Error).message, variant: "error" });
    } finally {
      setActivationLoading(false);
    }
  };

  // Reprocess entities
  const handleReprocessEntities = async () => {
    setIsReprocessing(true);
    try {
      const response = await api.axiosInstance.post<string>(
        `/v1/memories/extract-all-entities`, undefined, { timeout: 0 }
      );
      toast({ title: t("relationships.reprocessOk"), description: response.data || t("relationships.reprocessOkDesc"), variant: "success" });
      fetchKnowledgeGraph();
    } catch (err) {
      toast({ title: t("relationships.reprocessError"), description: (err as Error).message, variant: "error" });
    } finally {
      setIsReprocessing(false);
    }
  };

  // Create relationship
  const handleCreateRelationship = async () => {
    if (!selectedMemory || !targetMemory) return;
    try {
      await api.axiosInstance.post("/v1/relationships", {
        fromMemoryId: selectedMemory.id, toMemoryId: targetMemory.id,
        type: relationshipType, strength: 0.5,
      });
      toast({ title: t("relationships.relationshipCreated"), variant: "success" });
      setShowCreateDialog(false);
      refetch();
      setSelectedMemory(null);
      setTargetMemory(null);
      setHighlightedMemory(null);
    } catch (err) {
      toast({ title: t("relationships.genericError"), description: (err as Error).message, variant: "error" });
    }
  };

  // Delete relationship
  const handleDeleteRelationship = async (relationshipId: string) => {
    try {
      await api.axiosInstance.delete("/v1/relationships/between", { data: { relationshipId } });
      toast({ title: t("relationships.relationshipRemoved"), variant: "success" });
      refetch();
    } catch (err) {
      toast({ title: t("relationships.genericError"), description: (err as Error).message, variant: "error" });
    }
  };

  // Build cytoscape elements from knowledge graph + communities
  const cyElements = (() => {
    if (!knowledgeGraph) return [];
    const communityMap: Record<string, number> = {};
    communities.forEach((c, idx) => {
      c.members?.forEach(m => { communityMap[m] = idx; });
    });

    const nodes = knowledgeGraph.nodes.map(n => {
      const config = getEntityConfig(n.type);
      const communityIdx = communityMap[n.id];
      const color = communityIdx !== undefined ? COMMUNITY_COLORS[communityIdx % COMMUNITY_COLORS.length] : config.cyColor;
      return {
        data: {
          id: n.id, label: n.name, type: n.type,
          color, communityIdx: communityIdx ?? -1,
        },
      };
    });

    const edges = knowledgeGraph.edges.map(e => ({
      data: {
        id: e.id, source: e.sourceId, target: e.targetId,
        label: e.type,
      },
    }));

    return [...nodes, ...edges];
  })();

  const cyStylesheet = [
    {
      selector: "node",
      style: {
        "background-color": "data(color)",
        label: "data(label)",
        "font-size": "10px",
        "text-valign": "bottom" as const,
        "text-halign": "center" as const,
        width: 30,
        height: 30,
        "text-margin-y": 5,
        color: "#374151",
        "text-max-width": "80px",
        "text-wrap": "ellipsis" as const,
      },
    },
    {
      selector: "edge",
      style: {
        width: 1.5,
        "line-color": "#D1D5DB",
        "target-arrow-color": "#9CA3AF",
        "target-arrow-shape": "triangle" as const,
        "curve-style": "bezier" as const,
        label: "data(label)",
        "font-size": "8px",
        "text-rotation": "autorotate" as const,
        color: "#9CA3AF",
      },
    },
    {
      selector: "node:selected",
      style: {
        "border-width": 3,
        "border-color": "#6366F1",
      },
    },
  ];

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-10 border-b bg-gradient-to-r from-brain-primary to-brain-accent text-white">
        <div className="px-4 py-[14px]">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-1.5 bg-white/20 rounded-lg backdrop-blur-sm">
                <Network className="h-5 w-5 text-white" />
              </div>
              <div>
                <h1 className="text-base font-bold leading-tight">{t("relationships.title")}</h1>
                <p className="text-xs text-white/80">{t("relationships.subtitle")}</p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" className="bg-white/20 border-white/30 text-white hover:bg-white/30"
                onClick={handleReprocessEntities} disabled={isReprocessing}>
                {isReprocessing ? <Spinner className="h-4 w-4" /> : <Zap className="h-4 w-4 mr-1" />}
                {isReprocessing ? t("relationships.processing") : t("relationships.reprocess")}
              </Button>
              <Button variant="outline" size="sm" className="bg-white/20 border-white/30 text-white hover:bg-white/30" onClick={refetch}>
                <RefreshCw className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8">
        {/* Tab Navigation */}
        <div className="flex gap-2 mb-6 flex-wrap">
          <Button variant={activeTab === "graph" ? "default" : "outline"}
            onClick={() => setActiveTab("graph")}
            className={activeTab === "graph" ? "bg-gradient-to-r from-brain-primary to-brain-accent" : ""}>
            <Brain className="h-4 w-4 mr-2" />
            {t("relationships.tabs.graph")}
          </Button>
          <Button variant={activeTab === "knowledge" ? "default" : "outline"}
            onClick={() => setActiveTab("knowledge")}
            className={activeTab === "knowledge" ? "bg-gradient-to-r from-brain-primary to-brain-accent" : ""}>
            <Network className="h-4 w-4 mr-2" />
            {t("relationships.tabs.knowledge")}
          </Button>
          <Button variant={activeTab === "memory" ? "default" : "outline"}
            onClick={() => setActiveTab("memory")}
            className={activeTab === "memory" ? "bg-gradient-to-r from-brain-primary to-brain-accent" : ""}>
            <ArrowRight className="h-4 w-4 mr-2" />
            {t("relationships.tabs.memory")}
          </Button>
        </div>

        {/* Interactive Graph Tab */}
        {activeTab === "graph" && (
          <>
            {/* NL Query Section */}
            <Card className="mb-6">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm flex items-center gap-2">
                  <MessageSquare className="h-4 w-4 text-brain-primary" />
                  {t("relationships.askGraph")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={nlQuestion}
                    onChange={(e) => setNlQuestion(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && handleNlQuery()}
                    placeholder={t("relationships.askPlaceholder")}
                    className="flex-1 h-10 rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brain-primary/50"
                  />
                  <Button onClick={handleNlQuery} disabled={nlLoading || !nlQuestion.trim()}>
                    {nlLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4 mr-1" />}
                    {t("relationships.ask")}
                  </Button>
                </div>
                {nlAnswer && (
                  <div className="mt-3 p-3 bg-accent rounded-md">
                    <p className="text-sm font-medium mb-1">{t("relationships.askResult")}</p>
                    {nlAnswer.cypher && (
                      <pre className="text-xs bg-background p-2 rounded mb-2 overflow-x-auto">{nlAnswer.cypher}</pre>
                    )}
                    <div className="text-sm">
                      {nlAnswer.answer || nlAnswer.result || JSON.stringify(nlAnswer.data || nlAnswer, null, 2)}
                    </div>
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Stats Row */}
            <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
              <Card>
                <CardContent className="pt-4">
                  <div className="text-2xl font-bold text-brain-primary">{knowledgeGraph?.totalNodes || 0}</div>
                  <p className="text-sm text-muted-foreground">{t("relationships.stats.entities")}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="pt-4">
                  <div className="text-2xl font-bold text-brain-accent">{knowledgeGraph?.totalEdges || 0}</div>
                  <p className="text-sm text-muted-foreground">{t("relationships.stats.relationships")}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="pt-4">
                  <div className="text-2xl font-bold text-green-600">{new Set(knowledgeGraph?.nodes.map(n => n.type) || []).size}</div>
                  <p className="text-sm text-muted-foreground">{t("relationships.stats.entityTypes")}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="pt-4">
                  <div className="text-2xl font-bold text-purple-600">{communities.length}</div>
                  <p className="text-sm text-muted-foreground">{t("relationships.stats.communities")}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="pt-4">
                  <div className="text-2xl font-bold text-orange-600">{totalElements}</div>
                  <p className="text-sm text-muted-foreground">{t("relationships.stats.manualConnections")}</p>
                </CardContent>
              </Card>
            </div>

            {/* Cytoscape Graph */}
            <Card className="mb-6">
              <CardHeader className="pb-2">
                <div className="flex items-center justify-between">
                  <CardTitle className="flex items-center gap-2">
                    <Brain className="h-5 w-5" />
                    {t("relationships.graphTitle")}
                  </CardTitle>
                  {communities.length > 0 && (
                    <div className="flex items-center gap-2 flex-wrap">
                      {communities.slice(0, 6).map((c, idx) => (
                        <span key={c.id} className="flex items-center gap-1 text-xs">
                          <span className="w-3 h-3 rounded-full" style={{ backgroundColor: COMMUNITY_COLORS[idx % COMMUNITY_COLORS.length] }} />
                          {t("relationships.community", { id: c.id, size: c.size })}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              </CardHeader>
              <CardContent>
                {isLoadingGraph ? (
                  <div className="h-[500px] flex items-center justify-center">
                    <Spinner /> <span className="ml-2 text-muted-foreground">{t("relationships.loadingGraph")}</span>
                  </div>
                ) : cyElements.length === 0 ? (
                  <div className="h-[500px] flex items-center justify-center text-center text-muted-foreground">
                    <div>
                      <Network className="h-16 w-16 mx-auto mb-4 opacity-50" />
                      <h3 className="text-lg font-semibold mb-2">{t("relationships.noEntities")}</h3>
                      <p className="max-w-md">{t("relationships.noEntitiesDesc")}</p>
                    </div>
                  </div>
                ) : (
                  <div className="h-[500px] border rounded-lg overflow-hidden">
                    <CytoscapeComponent
                      elements={cyElements}
                      stylesheet={cyStylesheet as any}
                      layout={{ name: "cose", animate: true, animationDuration: 500 } as any}
                      style={{ width: "100%", height: "100%" }}
                      cy={(cy: any) => { cyRef.current = cy; }}
                    />
                  </div>
                )}
              </CardContent>
            </Card>

            {/* Spreading Activation */}
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm flex items-center gap-2">
                  <Sparkles className="h-4 w-4 text-brain-accent" />
                  {t("relationships.activation")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex gap-2 mb-3">
                  <input
                    type="text"
                    value={activationSeedId}
                    onChange={(e) => setActivationSeedId(e.target.value)}
                    placeholder={t("relationships.activationPlaceholder")}
                    className="flex-1 h-10 rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brain-primary/50"
                  />
                  <Button onClick={handleActivation} disabled={activationLoading || !activationSeedId.trim()}>
                    {activationLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Sparkles className="h-4 w-4 mr-1" />}
                    {t("relationships.activate")}
                  </Button>
                </div>
                {activationResults && (
                  <div className="space-y-2">
                    {(Array.isArray(activationResults) ? activationResults : activationResults?.activations || []).map((item: any, idx: number) => (
                      <div key={idx} className="flex items-center justify-between p-2 bg-accent rounded-md">
                        <span className="text-sm truncate flex-1">{item.memoryId || item.id}</span>
                        <div className="flex items-center gap-2">
                          <div className="w-24 h-2 bg-gray-200 rounded-full overflow-hidden">
                            <div className="h-full bg-brain-primary rounded-full" style={{ width: `${((item.activation || item.score || 0) * 100)}%` }} />
                          </div>
                          <span className="text-xs text-muted-foreground w-12 text-right">
                            {((item.activation || item.score || 0) * 100).toFixed(1)}%
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </>
        )}

        {/* Knowledge Tab (existing entities list) */}
        {activeTab === "knowledge" && (
          <>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
              <Card>
                <CardContent className="pt-4">
                  <div className="text-2xl font-bold text-brain-primary">{knowledgeGraph?.totalNodes || 0}</div>
                  <p className="text-sm text-muted-foreground">{t("relationships.stats.entities")}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="pt-4">
                  <div className="text-2xl font-bold text-brain-accent">{knowledgeGraph?.totalEdges || 0}</div>
                  <p className="text-sm text-muted-foreground">{t("relationships.stats.relationships")}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="pt-4">
                  <div className="text-2xl font-bold text-green-600">{new Set(knowledgeGraph?.nodes.map(n => n.type) || []).size}</div>
                  <p className="text-sm text-muted-foreground">{t("relationships.stats.entityTypes")}</p>
                </CardContent>
              </Card>
              <Card>
                <CardContent className="pt-4">
                  <div className="text-2xl font-bold text-purple-600">{new Set(knowledgeGraph?.edges.map(e => e.type) || []).size}</div>
                  <p className="text-sm text-muted-foreground">{t("relationships.stats.relationTypes")}</p>
                </CardContent>
              </Card>
            </div>

            <Card className="mb-6">
              <CardHeader>
                <CardTitle className="flex items-center gap-2"><Users className="h-5 w-5" /> {t("relationships.extractedEntities")}</CardTitle>
              </CardHeader>
              <CardContent>
                {isLoadingGraph ? (
                  <div className="space-y-4">
                    {Array.from({ length: 3 }).map((_, i) => (
                      <div key={i} className="flex items-center gap-4 p-4 border rounded-lg">
                        <Skeleton variant="circular" width={40} height={40} />
                        <Skeleton variant="text" width="40%" />
                      </div>
                    ))}
                  </div>
                ) : !knowledgeGraph || knowledgeGraph.nodes.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Network className="h-16 w-16 mx-auto mb-4 opacity-50" />
                    <h3 className="text-lg font-semibold mb-2">{t("relationships.noEntities")}</h3>
                  </div>
                ) : (
                  <div className="space-y-3 max-h-[400px] overflow-y-auto">
                    {knowledgeGraph.nodes.map((entity) => {
                      const config = getEntityConfig(entity.type);
                      const Icon = config.icon;
                      return (
                        <div key={entity.id} className="flex items-center gap-4 p-3 border rounded-lg hover:bg-accent transition-colors">
                          <div className={`p-2 rounded-full ${config.bgColor}`}>
                            <Icon className={`h-4 w-4 ${config.color}`} />
                          </div>
                          <div className="flex-1">
                            <p className="font-medium">{entity.name}</p>
                            <p className="text-xs text-muted-foreground">{t("relationships.entityType", { type: entity.type })}</p>
                          </div>
                          <span className={`px-2 py-1 rounded-full text-xs font-medium ${config.bgColor} ${config.color}`}>{entity.type}</span>
                        </div>
                      );
                    })}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2"><ArrowRight className="h-5 w-5" /> {t("relationships.entityRelations")}</CardTitle>
              </CardHeader>
              <CardContent>
                {isLoadingGraph ? (
                  <div className="space-y-4">
                    {Array.from({ length: 3 }).map((_, i) => (
                      <div key={i} className="flex items-center gap-4 p-4 border rounded-lg">
                        <Skeleton variant="text" width="30%" /> <Skeleton variant="text" width="20%" /> <Skeleton variant="text" width="30%" />
                      </div>
                    ))}
                  </div>
                ) : !knowledgeGraph || knowledgeGraph.edges.length === 0 ? (
                  <div className="text-center py-8 text-muted-foreground">
                    <p className="text-sm">{t("relationships.noRelations")}</p>
                  </div>
                ) : (
                  <div className="space-y-3 max-h-[400px] overflow-y-auto">
                    {knowledgeGraph.edges.map((edge) => (
                      <div key={edge.id} className="flex items-center gap-3 p-3 border rounded-lg hover:bg-accent transition-colors">
                        <span className="font-medium text-sm">{edge.sourceName}</span>
                        <span className="px-2 py-1 rounded-full bg-brain-primary/10 text-brain-primary text-xs font-medium">{edge.type}</span>
                        <ArrowRight className="h-4 w-4 text-muted-foreground" />
                        <span className="font-medium text-sm">{edge.targetName}</span>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </>
        )}

        {/* Memory Relationships Tab */}
        {activeTab === "memory" && (
          <>
            <Card className="mb-6">
              <CardContent className="pt-6">
                <div className="flex gap-2">
                  <div className="flex-1">
                    <SearchInput value={searchQuery} onChange={setSearchQuery} placeholder={t("relationships.connectSearchPlaceholder")} />
                  </div>
                  <Button className="bg-gradient-to-r from-brain-primary to-brain-accent text-white">
                    <Search className="h-4 w-4 mr-2" /> {t("common.search")}
                  </Button>
                </div>
              </CardContent>
            </Card>

            {shouldSearch && (
              <Card className="mb-6">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2"><Search className="h-4 w-4" /> {t("relationships.selectSource")}</CardTitle>
                </CardHeader>
                <CardContent>
                  {isSearching ? (
                    <div className="flex justify-center py-4"><Spinner /></div>
                  ) : searchResults.length > 0 ? (
                    <div className="space-y-2 max-h-[300px] overflow-y-auto">
                      {searchResults.map((memory) => (
                        <div key={memory.id}
                          className={`flex items-center justify-between p-3 border rounded-lg cursor-pointer transition-colors
                            ${highlightedMemory?.id === memory.id ? 'bg-brain-primary/10 border-brain-primary' : 'hover:bg-accent'}`}
                          onClick={() => { setHighlightedMemory(memory); fetchConnectionsForMemory(memory.id); }}>
                          <div className="flex-1 min-w-0">
                            <p className="text-sm font-medium truncate">{memory.summary}</p>
                            <div className="flex items-center gap-2 mt-1">
                              <CategoryTag category={memory.category} />
                              <span className="text-xs text-muted-foreground">{memory.tags?.slice(0, 2).join(", ") || ""}</span>
                            </div>
                          </div>
                          {highlightedMemory?.id === memory.id && (
                            <div className="flex items-center gap-1 text-brain-primary">
                              <span className="text-xs font-medium">{t("relationships.selected")}</span>
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-center text-muted-foreground py-4">{t("relationships.noResultsFor", { query: debouncedSearchQuery })}</p>
                  )}
                </CardContent>
              </Card>
            )}

            {highlightedMemory && (
              <Card className="mb-6 border-brain-primary/30">
                <CardHeader>
                  <CardTitle className="flex items-center justify-between">
                    <span className="flex items-center gap-2"><Network className="h-5 w-5 text-brain-primary" /> {t("relationships.selectedMemory")}</span>
                    <Button className="bg-gradient-to-r from-brain-primary to-brain-accent text-white"
                      onClick={() => { setSelectedMemory(highlightedMemory); setShowCreateDialog(true); }}>
                      <Link2 className="h-4 w-4 mr-2" /> {t("relationships.connect")}
                    </Button>
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="p-4 border rounded-lg bg-accent/50 mb-4">
                    <p className="font-medium">{highlightedMemory.summary}</p>
                    <p className="text-sm text-muted-foreground mt-2 line-clamp-2">{highlightedMemory.content}</p>
                    <div className="flex items-center gap-2 mt-3">
                      <CategoryTag category={highlightedMemory.category} />
                      <span className="text-xs text-muted-foreground">{highlightedMemory.tags?.join(", ") || "-"}</span>
                    </div>
                  </div>
                  <h4 className="text-sm font-medium mb-2">{t("relationships.existingConnections")}</h4>
                  {isLoadingConnections ? (
                    <div className="flex justify-center py-4"><Spinner /></div>
                  ) : highlightedConnections.length > 0 ? (
                    <div className="space-y-2 max-h-[200px] overflow-y-auto">
                      {highlightedConnections.map((conn) => {
                        const isSource = conn.fromMemoryId === highlightedMemory.id;
                        const otherMemory = isSource ? conn.toMemory : conn.fromMemory;
                        return (
                          <div key={conn.id} className="flex items-center gap-2 p-2 border rounded hover:bg-accent transition-colors">
                            <ArrowRight className="h-3 w-3 text-muted-foreground" />
                            <span className="text-sm flex-1 truncate">{otherMemory?.summary || (isSource ? conn.toMemoryId : conn.fromMemoryId)}</span>
                            <span className="px-2 py-0.5 rounded-full bg-brain-primary/10 text-brain-primary text-xs">{conn.type}</span>
                          </div>
                        );
                      })}
                    </div>
                  ) : (
                    <p className="text-sm text-muted-foreground py-2">{t("relationships.noConnections")}</p>
                  )}
                </CardContent>
              </Card>
            )}

            <Card>
              <CardHeader>
                <CardTitle>{t("relationships.connectedMemories")}</CardTitle>
              </CardHeader>
              <CardContent>
                {isLoading ? (
                  <div className="space-y-4">
                    {Array.from({ length: 3 }).map((_, i) => (
                      <div key={i} className="flex items-center gap-4 p-4 border rounded-lg">
                        <Skeleton variant="circular" width={40} height={40} />
                        <Skeleton variant="text" width="40%" />
                      </div>
                    ))}
                  </div>
                ) : relationships.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Network className="h-16 w-16 mx-auto mb-4 opacity-50" />
                    <h3 className="text-lg font-semibold mb-2">{t("relationships.noManualConnections")}</h3>
                    <p>{t("relationships.useSearchAbove")}</p>
                  </div>
                ) : (
                  <div className="space-y-4 max-h-[500px] overflow-y-auto">
                    {relationships.map((rel) => (
                      <RelationshipItem key={rel.id} relationship={rel} onDelete={handleDeleteRelationship} />
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </>
        )}
      </main>

      {/* Create Relationship Dialog */}
      <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
        <DialogContent className="sm:max-w-[700px] max-h-[85vh] overflow-hidden flex flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Link2 className="h-5 w-5 text-brain-primary" /> {t("relationships.dialogTitle")}
            </DialogTitle>
          </DialogHeader>
          <div className="flex-1 overflow-y-auto space-y-6 py-4">
            {selectedMemory && (
              <div>
                <label className="text-sm font-medium text-muted-foreground mb-2 block">{t("relationships.sourceMemory")}</label>
                <div className="p-4 border rounded-lg bg-brain-primary/5 border-brain-primary/30">
                  <p className="font-medium">{selectedMemory.summary}</p>
                  <div className="flex items-center gap-2 mt-2">
                    <CategoryTag category={selectedMemory.category} />
                  </div>
                </div>
              </div>
            )}
            <div>
              <label className="text-sm font-medium text-muted-foreground mb-2 block">{t("relationships.type")}</label>
              <select className="w-full h-10 rounded-md border border-input bg-background px-3 py-2 text-sm"
                value={relationshipType} onChange={(e) => setRelationshipType(e.target.value)}>
                {RELATIONSHIP_TYPES.map((type) => (<option key={type.value} value={type.value}>{type.label}</option>))}
              </select>
            </div>
            <div>
              <label className="text-sm font-medium text-muted-foreground mb-2 block">{t("relationships.searchTarget")}</label>
              <SearchInput value={dialogSearchQuery} onChange={setDialogSearchQuery} placeholder={t("relationships.searchTargetPlaceholder")} />
              {isDialogSearching ? (
                <div className="flex justify-center py-4"><Spinner /></div>
              ) : dialogSearchResults.length > 0 ? (
                <div className="mt-3 space-y-2 max-h-[200px] overflow-y-auto border rounded-lg p-2">
                  {dialogSearchResults.map((memory) => (
                    <div key={memory.id}
                      className={`p-3 border rounded-lg cursor-pointer transition-colors
                        ${targetMemory?.id === memory.id ? 'bg-brain-accent/10 border-brain-accent' : 'hover:bg-accent'}`}
                      onClick={() => setTargetMemory(memory)}>
                      <p className="text-sm font-medium">{memory.summary}</p>
                      <div className="flex items-center gap-2 mt-1">
                        <CategoryTag category={memory.category} />
                      </div>
                    </div>
                  ))}
                </div>
              ) : dialogSearchQuery.length >= 2 ? (
                <p className="text-sm text-muted-foreground mt-3 text-center py-4">{t("memory.notFound")}</p>
              ) : null}
            </div>
            {targetMemory && (
              <div>
                <label className="text-sm font-medium text-muted-foreground mb-2 block">{t("relationships.targetMemory")}</label>
                <div className="p-4 border rounded-lg bg-brain-accent/5 border-brain-accent/30">
                  <p className="font-medium">{targetMemory.summary}</p>
                </div>
              </div>
            )}
            {selectedMemory && targetMemory && (
              <div className="p-4 border-2 border-dashed rounded-lg bg-accent/30">
                <p className="text-sm text-center text-muted-foreground mb-2">{t("relationships.preview")}</p>
                <div className="flex items-center justify-center gap-3 flex-wrap">
                  <span className="font-medium text-sm bg-brain-primary/10 px-3 py-1 rounded">{selectedMemory.summary.substring(0, 30)}...</span>
                  <span className="px-3 py-1 rounded-full bg-brain-primary text-white text-xs font-medium">
                    {RELATIONSHIP_TYPES.find(t => t.value === relationshipType)?.label || relationshipType}
                  </span>
                  <ArrowRight className="h-4 w-4 text-muted-foreground" />
                  <span className="font-medium text-sm bg-brain-accent/10 px-3 py-1 rounded">{targetMemory.summary.substring(0, 30)}...</span>
                </div>
              </div>
            )}
          </div>
          <DialogFooter className="border-t pt-4">
            <Button variant="outline" onClick={() => setShowCreateDialog(false)}>{t("common.cancel")}</Button>
            <Button className="bg-gradient-to-r from-brain-primary to-brain-accent text-white"
              onClick={handleCreateRelationship} disabled={!selectedMemory || !targetMemory}>
              <Link2 className="h-4 w-4 mr-2" /> {t("relationships.createConnection")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

interface RelationshipItemProps {
  relationship: Relationship;
  onDelete: (id: string) => void;
}

function RelationshipItem({ relationship, onDelete }: RelationshipItemProps) {
  const { t } = useTranslation();
  const typeLabels: Record<string, string> = {
    RELATED: t("relationships.types.RELATED"),
    DEPENDS_ON: t("relationships.types.DEPENDS_ON"),
    DEPENDENT_OF: t("relationships.types.DEPENDENT_OF_LONG"),
    CONTRADICTS: t("relationships.types.CONTRADICTS"),
    SIMILAR_TO: t("relationships.types.SIMILAR_TO"),
    EXTENDS: t("relationships.types.EXTENDS"),
  };
  return (
    <div className="flex items-center justify-between p-4 border rounded-lg hover:bg-accent transition-colors">
      <div className="flex items-center gap-4">
        <div className="p-2 bg-primary/10 rounded-full"><Network className="h-4 w-4 text-primary" /></div>
        <div className="flex-1">
          {relationship.fromMemory && relationship.toMemory ? (
            <>
              <div className="text-sm"><span className="font-medium">{relationship.fromMemory.summary}</span></div>
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <ArrowRight className="h-3 w-3" />
                <span>{typeLabels[relationship.type] || relationship.type}</span>
                <ArrowRight className="h-3 w-3" />
                <span className="font-medium text-foreground">{relationship.toMemory.summary}</span>
              </div>
            </>
          ) : (
            <div className="text-sm text-muted-foreground">{t("relationships.relationshipId", { id: relationship.id })}</div>
          )}
          <div className="flex items-center gap-4 text-xs text-muted-foreground">
            <span>{t("relationships.strength", { pct: Math.round((relationship.strength || 0) * 100) })}</span>
          </div>
        </div>
        <Button variant="ghost" size="icon" onClick={() => onDelete(relationship.id)}>
          <Trash2 className="h-4 w-4 text-destructive" />
        </Button>
      </div>
    </div>
  );
}
