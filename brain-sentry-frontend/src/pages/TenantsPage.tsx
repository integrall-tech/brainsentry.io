import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Building2,
  Plus,
  Search,
  Edit,
  Trash2,
  Users,
  Database,
  Settings,
  RefreshCw,
  Key,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui";
import { Button } from "@/components/ui/button";
import { Input, SearchInput } from "@/components/ui/filter";
import { Pagination, SimplePagination } from "@/components/ui/pagination";
import { Spinner, Skeleton } from "@/components/ui/spinner";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useFetch } from "@/hooks";
import { useToast } from "@/components/ui/toast";
import { useAuth } from "@/contexts/AuthContext";

import { API_BASE_URL as API_URL } from "@/lib/api/client";

interface Tenant {
  id: string;
  name: string;
  slug: string;
  active: boolean;
  maxMemories?: number;
  maxUsers?: number;
  createdAt: string;
  settings?: Record<string, unknown>;
}

interface TenantStats {
  tenantId: string;
  memoryCount: number;
  userCount: number;
  relationshipCount: number;
}

interface CreateTenantRequest {
  name: string;
  slug: string;
  maxMemories?: number;
  maxUsers?: number;
  settings?: Record<string, unknown>;
  active?: boolean;
}

interface UpdateTenantRequest {
  name?: string;
  active?: boolean;
  maxMemories?: number;
  maxUsers?: number;
  settings?: Record<string, unknown>;
}

export function TenantsPage() {
  const { t, i18n } = useTranslation();
  const { user: currentUser } = useAuth();
  const { toast } = useToast();
  // Sem fallback: header vazio é ignorado pelo backend, que usa o JWT claim.
  const tenantId = currentUser?.tenantId ?? "";
  const dateLocale = i18n.language === "en" ? "en-US" : "pt-BR";

  // State
  const [searchQuery, setSearchQuery] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [selectedTenant, setSelectedTenant] = useState<Tenant | null>(null);
  const [formData, setFormData] = useState<CreateTenantRequest>({
    name: "",
    slug: "",
    maxMemories: 1000,
    maxUsers: 10,
  });

  // Fetch tenants
  const {
    data: tenantsData,
    isLoading,
    error,
    refetch,
  } = useFetch<{ content: Tenant[]; totalElements: number }>(
    `${API_URL}/v1/tenants?page=${page - 1}&size=${pageSize}`
  );

  // Fetch stats for each tenant
  const { data: statsData } = useFetch<TenantStats[]>(
    `${API_URL}/v1/tenants/stats`
  );

  const tenants = tenantsData?.content || [];
  const totalElements = tenantsData?.totalElements || 0;
  const totalPages = Math.ceil(totalElements / pageSize);

  const statsMap = new Map(statsData?.map((s) => [s.tenantId, s]) || []);

  // Filter tenants by search
  const filteredTenants = tenants.filter(
    (t) =>
      t.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      t.slug.toLowerCase().includes(searchQuery.toLowerCase())
  );

  // Generate slug from name
  const generateSlug = (name: string) => {
    return name
      .toLowerCase()
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "")
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "");
  };

  // Create tenant
  const handleCreateTenant = async () => {
    try {
      const response = await fetch(`${API_URL}/v1/tenants`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Tenant-ID": tenantId,
        },
        body: JSON.stringify(formData),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || "Failed to create tenant");
      }

      toast({
        title: t("tenants.tenantCreated"),
        description: t("tenants.tenantCreatedDesc", { name: formData.name }),
        variant: "success",
      });

      setShowCreateDialog(false);
      resetForm();
      refetch?.();
    } catch (err) {
      toast({
        title: t("common.error"),
        description: (err as Error).message || t("tenants.createError"),
        variant: "error",
      });
    }
  };

  // Update tenant
  const handleUpdateTenant = async () => {
    if (!selectedTenant) return;

    try {
      const response = await fetch(`${API_URL}/v1/tenants/${selectedTenant.id}`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          "X-Tenant-ID": tenantId,
        },
        body: JSON.stringify({
          name: formData.name,
          active: formData.active ?? true,
          maxMemories: formData.maxMemories,
          maxUsers: formData.maxUsers,
        } as UpdateTenantRequest),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || "Failed to update tenant");
      }

      toast({
        title: t("tenants.tenantUpdated"),
        description: t("tenants.tenantUpdatedDesc", { name: selectedTenant.name }),
        variant: "success",
      });

      setShowEditDialog(false);
      setSelectedTenant(null);
      refetch?.();
    } catch (err) {
      toast({
        title: t("common.error"),
        description: (err as Error).message || t("tenants.updateError"),
        variant: "error",
      });
    }
  };

  // Delete tenant
  const handleDeleteTenant = async (tenantIdToDelete: string, name: string) => {
    if (
      !confirm(t("tenants.deleteConfirm", { name }))
    ) {
      return;
    }

    try {
      const response = await fetch(`${API_URL}/v1/tenants/${tenantIdToDelete}`, {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json",
          "X-Tenant-ID": tenantId,
        },
      });

      if (!response.ok) {
        throw new Error("Failed to delete tenant");
      }

      toast({
        title: t("tenants.tenantDeleted"),
        description: t("tenants.tenantDeletedDesc", { name }),
        variant: "success",
      });

      refetch?.();
    } catch (err) {
      toast({
        title: t("common.error"),
        description: t("tenants.deleteError"),
        variant: "error",
      });
    }
  };

  const resetForm = () => {
    setFormData({
      name: "",
      slug: "",
      maxMemories: 1000,
      maxUsers: 10,
    });
  };

  const openEditDialog = (tenant: Tenant) => {
    setSelectedTenant(tenant);
    setFormData({
      name: tenant.name,
      slug: tenant.slug,
      maxMemories: tenant.maxMemories,
      maxUsers: tenant.maxUsers,
      active: tenant.active,
    });
    setShowEditDialog(true);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString(dateLocale);
  };

  const getTenantStats = (tenant: Tenant) => {
    return statsMap.get(tenant.id);
  };

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-10 border-b bg-gradient-to-r from-brain-primary to-brain-accent text-white">
        <div className="px-4 py-[14px]">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-1.5 bg-white/20 rounded-lg backdrop-blur-sm">
                <Building2 className="h-5 w-5 text-white" />
              </div>
              <div>
                <h1 className="text-base font-bold leading-tight">{t("tenants.title")}</h1>
                <p className="text-xs text-white/80">
                  {t("tenants.subtitle")}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" className="bg-white/20 border-white/30 text-white hover:bg-white/30" onClick={() => refetch?.()}>
                <RefreshCw className="h-4 w-4" />
              </Button>
              <Button className="bg-white text-brain-primary hover:bg-white/90" onClick={() => setShowCreateDialog(true)}>
                <Plus className="h-4 w-4 mr-2" />
                {t("tenants.newTenant")}
              </Button>
            </div>
          </div>
        </div>
      </header>

      <main className="container mx-auto px-4 py-8">
        {/* Search */}
        <div className="mb-6">
          <SearchInput
            value={searchQuery}
            onChange={setSearchQuery}
            placeholder={t("tenants.searchPlaceholder")}
          />
        </div>

        {/* Tenants Grid */}
        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {Array.from({ length: 6 }).map((_, i) => (
              <Card key={i}>
                <CardContent className="p-6">
                  <Skeleton variant="text" width="60%" />
                  <Skeleton variant="text" width="40%" />
                  <Skeleton variant="text" width="80%" />
                </CardContent>
              </Card>
            ))}
          </div>
        ) : filteredTenants.length === 0 ? (
          <Card>
            <CardContent className="text-center py-12 text-muted-foreground">
              <Building2 className="h-16 w-16 mx-auto mb-4 opacity-50" />
              <h3 className="text-lg font-semibold mb-2">
                {searchQuery ? t("tenants.notFound") : t("tenants.noneRegistered")}
              </h3>
              <p className="mb-4">
                {searchQuery
                  ? t("tenants.tryOther")
                  : t("tenants.startAdding")}
              </p>
              {!searchQuery && (
                <Button className="bg-gradient-to-r from-brain-primary to-brain-accent hover:from-brain-primary-dark hover:to-brain-accent-dark text-white" onClick={() => setShowCreateDialog(true)}>
                  <Plus className="h-4 w-4 mr-2" />
                  {t("tenants.addTenant")}
                </Button>
              )}
            </CardContent>
          </Card>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {filteredTenants.map((tenant) => {
              const stats = getTenantStats(tenant);
              const isCurrentTenant = tenant.id === tenantId;

              return (
                <Card
                  key={tenant.id}
                  className={`transition-all hover:shadow-md ${
                    isCurrentTenant ? "border-primary" : ""
                  }`}
                >
                  <CardHeader className="pb-3">
                    <div className="flex items-start justify-between">
                      <div className="flex items-center gap-3">
                        <div className="p-2 bg-primary/10 rounded-lg">
                          <Building2 className="h-5 w-5 text-primary" />
                        </div>
                        <div>
                          <CardTitle className="text-lg">{tenant.name}</CardTitle>
                          <p className="text-xs text-muted-foreground font-mono">
                            @{tenant.slug}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-1">
                        <div
                          className={`w-2 h-2 rounded-full ${
                            tenant.active ? "bg-green-500" : "bg-red-500"
                          }`}
                        />
                      </div>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    {/* Stats */}
                    {stats && (
                      <div className="grid grid-cols-3 gap-2 text-center">
                        <div className="p-2 bg-muted/50 rounded-lg">
                          <Database className="h-4 w-4 mx-auto mb-1 text-muted-foreground" />
                          <p className="text-lg font-semibold">{stats.memoryCount}</p>
                          <p className="text-xs text-muted-foreground">{t("tenants.memories")}</p>
                        </div>
                        <div className="p-2 bg-muted/50 rounded-lg">
                          <Users className="h-4 w-4 mx-auto mb-1 text-muted-foreground" />
                          <p className="text-lg font-semibold">{stats.userCount}</p>
                          <p className="text-xs text-muted-foreground">{t("tenants.users")}</p>
                        </div>
                        <div className="p-2 bg-muted/50 rounded-lg">
                          <Key className="h-4 w-4 mx-auto mb-1 text-muted-foreground" />
                          <p className="text-lg font-semibold">{stats.relationshipCount}</p>
                          <p className="text-xs text-muted-foreground">{t("tenants.relations")}</p>
                        </div>
                      </div>
                    )}

                    {/* Limits */}
                    <div className="space-y-2 text-sm">
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">{t("tenants.memoryLimit")}</span>
                        <span className="font-medium">
                          {tenant.maxMemories || t("tenants.unlimited")}
                        </span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">{t("tenants.userLimit")}</span>
                        <span className="font-medium">{tenant.maxUsers || t("tenants.unlimited")}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-muted-foreground">{t("tenants.createdAt")}</span>
                        <span className="font-medium text-xs">
                          {formatDate(tenant.createdAt)}
                        </span>
                      </div>
                    </div>

                    {/* Actions */}
                    <div className="flex gap-2 pt-2 border-t">
                      <Button
                        variant="outline"
                        size="sm"
                        className="flex-1"
                        onClick={() => openEditDialog(tenant)}
                      >
                        <Edit className="h-3 w-3 mr-1" />
                        {t("common.edit")}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => handleDeleteTenant(tenant.id, tenant.name)}
                        disabled={isCurrentTenant}
                      >
                        <Trash2 className="h-3 w-3 text-destructive" />
                      </Button>
                    </div>

                    {isCurrentTenant && (
                      <div className="text-xs text-center text-primary font-medium bg-primary/10 py-1 rounded">
                        {t("tenants.currentTenant")}
                      </div>
                    )}
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="mt-6">
            <SimplePagination
              currentPage={page}
              totalPages={totalPages}
              onPageChange={setPage}
            />
          </div>
        )}
      </main>

      {/* Create Tenant Dialog */}
      <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>{t("tenants.newTenant")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 px-6 py-4">
            <div>
              <label className="text-sm font-medium mb-2">{t("tenants.name")}</label>
              <Input
                type="text"
                value={formData.name}
                onChange={(e) => {
                  const name = e.target.value;
                  setFormData({
                    ...formData,
                    name,
                    slug: generateSlug(name),
                  });
                }}
                placeholder={t("tenants.orgPlaceholder")}
              />
            </div>
            <div>
              <label className="text-sm font-medium mb-2">{t("tenants.slugLabel")}</label>
              <Input
                type="text"
                value={formData.slug}
                onChange={(e) =>
                  setFormData({ ...formData, slug: e.target.value.toLowerCase() })
                }
                placeholder={t("tenants.slugPlaceholder")}
              />
              <p className="text-xs text-muted-foreground mt-1">
                {t("tenants.slugHint")}
              </p>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium mb-2">{t("tenants.maxMemories")}</label>
                <Input
                  type="number"
                  value={formData.maxMemories || ""}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      maxMemories: Number(e.target.value) || undefined,
                    })
                  }
                  placeholder="1000"
                />
              </div>
              <div>
                <label className="text-sm font-medium mb-2">{t("tenants.maxUsers")}</label>
                <Input
                  type="number"
                  value={formData.maxUsers || ""}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      maxUsers: Number(e.target.value) || undefined,
                    })
                  }
                  placeholder="10"
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowCreateDialog(false)}>
              {t("common.cancel")}
            </Button>
            <Button className="bg-gradient-to-r from-brain-primary to-brain-accent hover:from-brain-primary-dark hover:to-brain-accent-dark text-white" onClick={handleCreateTenant} disabled={!formData.name || !formData.slug}>
              {t("tenants.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Tenant Dialog */}
      <Dialog open={showEditDialog} onOpenChange={setShowEditDialog}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>{t("tenants.editTenant")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 px-6 py-4">
            <div>
              <label className="text-sm font-medium mb-2">{t("tenants.name")}</label>
              <Input
                type="text"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              />
            </div>
            <div>
              <label className="text-sm font-medium mb-2">Slug</label>
              <Input type="text" value={formData.slug} disabled />
              <p className="text-xs text-muted-foreground mt-1">
                {t("tenants.slugLocked")}
              </p>
            </div>
            <div>
              <label className="text-sm font-medium mb-2">{t("common.status")}</label>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => setFormData({ ...formData, active: true })}
                  className={`px-3 py-1.5 rounded-full text-sm transition-colors ${
                    formData.active !== false
                      ? "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  {t("common.active")}
                </button>
                <button
                  type="button"
                  onClick={() => setFormData({ ...formData, active: false })}
                  className={`px-3 py-1.5 rounded-full text-sm transition-colors ${
                    formData.active === false
                      ? "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400"
                      : "bg-muted hover:bg-muted/80"
                  }`}
                >
                  {t("common.inactive")}
                </button>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium mb-2">{t("tenants.maxMemories")}</label>
                <Input
                  type="number"
                  value={formData.maxMemories || ""}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      maxMemories: Number(e.target.value) || undefined,
                    })
                  }
                />
              </div>
              <div>
                <label className="text-sm font-medium mb-2">{t("tenants.maxUsers")}</label>
                <Input
                  type="number"
                  value={formData.maxUsers || ""}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      maxUsers: Number(e.target.value) || undefined,
                    })
                  }
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowEditDialog(false)}>
              {t("common.cancel")}
            </Button>
            <Button className="bg-gradient-to-r from-brain-primary to-brain-accent hover:from-brain-primary-dark hover:to-brain-accent-dark text-white" onClick={handleUpdateTenant}>{t("tenants.saveChanges")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
