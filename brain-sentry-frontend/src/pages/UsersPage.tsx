import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Users,
  UserPlus,
  Search,
  Edit,
  Trash2,
  Mail,
  Shield,
  Clock,
  Check,
  X,
  MoreVertical,
  RefreshCw,
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

interface User {
  id: string;
  email: string;
  name?: string;
  tenantId: string;
  roles?: string[];
  active: boolean;
  createdAt: string;
  lastLoginAt?: string;
}

interface CreateUserRequest {
  email: string;
  name?: string;
  password: string;
  tenantId?: string;
  roles?: string[];
  active?: boolean;
}

interface UpdateUserRequest {
  name?: string;
  active?: boolean;
  roles?: string[];
}

const ROLE_KEYS = ["USER", "ADMIN", "MODERATOR"] as const;

const ROLE_COLORS: Record<string, string> = {
  ADMIN: "bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400",
  MODERATOR: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
  USER: "bg-gray-100 text-gray-700 dark:bg-gray-900/30 dark:text-gray-400",
};

export function UsersPage() {
  const { t, i18n } = useTranslation();
  const { user: currentUser } = useAuth();
  const { toast } = useToast();
  // Sem fallback: header vazio é ignorado pelo backend, que usa o JWT claim.
  const tenantId = currentUser?.tenantId ?? "";
  const dateLocale = i18n.language === "en" ? "en-US" : "pt-BR";
  const ROLE_OPTIONS = ROLE_KEYS.map((k) => ({ value: k, label: t(`users.roleOptions.${k}`) }));

  // State
  const [searchQuery, setSearchQuery] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [selectedUser, setSelectedUser] = useState<User | null>(null);
  const [formData, setFormData] = useState<CreateUserRequest>({
    email: "",
    name: "",
    password: "",
    tenantId,
    roles: ["USER"],
  });

  // Fetch users
  const {
    data: usersData,
    isLoading,
    error,
    refetch,
  } = useFetch<{ content: User[]; totalElements: number }>(
    `${API_URL}/v1/users?page=${page - 1}&size=${pageSize}`
  );

  const users = usersData?.content || [];
  const totalElements = usersData?.totalElements || 0;
  const totalPages = Math.ceil(totalElements / pageSize);

  // Filter users by search
  const filteredUsers = users.filter(
    (u) =>
      u.email.toLowerCase().includes(searchQuery.toLowerCase()) ||
      u.name?.toLowerCase().includes(searchQuery.toLowerCase())
  );

  // Create user
  const handleCreateUser = async () => {
    try {
      const response = await fetch(`${API_URL}/v1/users`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Tenant-ID": tenantId,
        },
        body: JSON.stringify(formData),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || "Failed to create user");
      }

      toast({
        title: t("users.userCreated"),
        description: t("users.userCreatedDesc", { email: formData.email }),
        variant: "success",
      });

      setShowCreateDialog(false);
      resetForm();
      refetch?.();
    } catch (err) {
      toast({
        title: t("common.error"),
        description: (err as Error).message || t("users.createUserError"),
        variant: "error",
      });
    }
  };

  // Update user
  const handleUpdateUser = async () => {
    if (!selectedUser) return;

    try {
      const response = await fetch(`${API_URL}/v1/users/${selectedUser.id}`, {
        method: "PATCH",
        headers: {
          "Content-Type": "application/json",
          "X-Tenant-ID": tenantId,
        },
        body: JSON.stringify({
          name: formData.name,
          active: formData.active ?? true,
          roles: formData.roles,
        } as UpdateUserRequest),
      });

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.message || "Failed to update user");
      }

      toast({
        title: t("users.userUpdated"),
        description: t("users.userUpdatedDesc", { email: selectedUser.email }),
        variant: "success",
      });

      setShowEditDialog(false);
      setSelectedUser(null);
      refetch?.();
    } catch (err) {
      toast({
        title: t("common.error"),
        description: (err as Error).message || t("users.updateUserError"),
        variant: "error",
      });
    }
  };

  // Delete user
  const handleDeleteUser = async (userId: string, email: string) => {
    if (!confirm(t("users.deleteConfirm", { email }))) {
      return;
    }

    try {
      const response = await fetch(`${API_URL}/v1/users/${userId}`, {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json",
          "X-Tenant-ID": tenantId,
        },
      });

      if (!response.ok) {
        throw new Error("Failed to delete user");
      }

      toast({
        title: t("users.userDeleted"),
        description: t("users.userDeletedDesc", { email }),
        variant: "success",
      });

      refetch?.();
    } catch (err) {
      toast({
        title: t("common.error"),
        description: t("users.deleteUserError"),
        variant: "error",
      });
    }
  };

  const resetForm = () => {
    setFormData({
      email: "",
      name: "",
      password: "",
      tenantId,
      roles: ["USER"],
    });
  };

  const openEditDialog = (user: User) => {
    setSelectedUser(user);
    setFormData({
      email: user.email,
      name: user.name,
      password: "",
      tenantId: user.tenantId,
      roles: user.roles || ["USER"],
      active: user.active,
    });
    setShowEditDialog(true);
  };

  const toggleRole = (role: string) => {
    const currentRoles = formData.roles || [];
    const newRoles = currentRoles.includes(role)
      ? currentRoles.filter((r) => r !== role)
      : [...currentRoles, role];
    setFormData({ ...formData, roles: newRoles });
  };

  const formatDate = (dateString?: string) => {
    if (!dateString) return "-";
    return new Date(dateString).toLocaleString(dateLocale);
  };

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="sticky top-0 z-10 border-b bg-gradient-to-r from-brain-primary to-brain-accent text-white">
        <div className="px-4 py-[14px]">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-1.5 bg-white/20 rounded-lg backdrop-blur-sm">
                <Users className="h-5 w-5 text-white" />
              </div>
              <div>
                <h1 className="text-base font-bold leading-tight">{t("users.title")}</h1>
                <p className="text-xs text-white/80">
                  {t("users.subtitle")}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="outline" size="sm" className="bg-white/20 border-white/30 text-white hover:bg-white/30" onClick={() => refetch?.()}>
                <RefreshCw className="h-4 w-4" />
              </Button>
              <Button className="bg-white text-brain-primary hover:bg-white/90" onClick={() => setShowCreateDialog(true)}>
                <UserPlus className="h-4 w-4 mr-2" />
                {t("users.newUser")}
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
            placeholder={t("users.searchPlaceholder")}
          />
        </div>

        {/* Users List */}
        <Card>
          <CardHeader>
            <CardTitle>
              {totalElements === 1 ? t("users.countSingular", { count: totalElements }) : t("users.count", { count: totalElements })}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="space-y-4">
                {Array.from({ length: 5 }).map((_, i) => (
                  <div key={i} className="flex items-center gap-4 p-4 border rounded-lg">
                    <Skeleton variant="circular" width={40} height={40} />
                    <Skeleton variant="text" width="30%" />
                    <Skeleton variant="text" width="20%" />
                  </div>
                ))}
              </div>
            ) : filteredUsers.length === 0 ? (
              <div className="text-center py-12 text-muted-foreground">
                <Users className="h-16 w-16 mx-auto mb-4 opacity-50" />
                <h3 className="text-lg font-semibold mb-2">
                  {searchQuery ? t("users.notFound") : t("users.noneRegistered")}
                </h3>
                <p className="mb-4">
                  {searchQuery
                    ? t("users.tryOther")
                    : t("users.startAdding")}
                </p>
                {!searchQuery && (
                  <Button className="bg-gradient-to-r from-brain-primary to-brain-accent hover:from-brain-primary-dark hover:to-brain-accent-dark text-white" onClick={() => setShowCreateDialog(true)}>
                    <UserPlus className="h-4 w-4 mr-2" />
                    {t("users.addUser")}
                  </Button>
                )}
              </div>
            ) : (
              <>
                <div className="overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr className="border-b text-left text-sm text-muted-foreground">
                        <th className="p-4">{t("users.columns.user")}</th>
                        <th className="p-4">{t("users.columns.roles")}</th>
                        <th className="p-4">{t("users.columns.status")}</th>
                        <th className="p-4">{t("users.columns.lastAccess")}</th>
                        <th className="p-4">{t("users.columns.createdAt")}</th>
                        <th className="p-4 text-right">{t("users.columns.actions")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredUsers.map((user) => (
                        <tr key={user.id} className="border-b hover:bg-muted/50">
                          <td className="p-4">
                            <div className="flex items-center gap-3">
                              <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center">
                                <Mail className="h-4 w-4 text-primary" />
                              </div>
                              <div>
                                <p className="font-medium">{user.name || user.email}</p>
                                <p className="text-sm text-muted-foreground">{user.email}</p>
                              </div>
                            </div>
                          </td>
                          <td className="p-4">
                            <div className="flex gap-1 flex-wrap">
                              {(user.roles || ["USER"]).map((role) => (
                                <span
                                  key={role}
                                  className={`text-xs px-2 py-1 rounded-full ${ROLE_COLORS[role] || ROLE_COLORS.USER}`}
                                >
                                  {role.toLowerCase()}
                                </span>
                              ))}
                            </div>
                          </td>
                          <td className="p-4">
                            {user.active ? (
                              <span className="flex items-center gap-1 text-green-600 dark:text-green-400 text-sm">
                                <Check className="h-3 w-3" />
                                {t("common.active")}
                              </span>
                            ) : (
                              <span className="flex items-center gap-1 text-red-600 dark:text-red-400 text-sm">
                                <X className="h-3 w-3" />
                                {t("common.inactive")}
                              </span>
                            )}
                          </td>
                          <td className="p-4 text-sm text-muted-foreground">
                            {formatDate(user.lastLoginAt)}
                          </td>
                          <td className="p-4 text-sm text-muted-foreground">
                            {formatDate(user.createdAt)}
                          </td>
                          <td className="p-4">
                            <div className="flex items-center justify-end gap-2">
                              <Button
                                variant="ghost"
                                size="icon"
                                onClick={() => openEditDialog(user)}
                              >
                                <Edit className="h-4 w-4" />
                              </Button>
                              <Button
                                variant="ghost"
                                size="icon"
                                onClick={() => handleDeleteUser(user.id, user.email)}
                                disabled={user.id === currentUser?.id}
                              >
                                <Trash2 className="h-4 w-4 text-destructive" />
                              </Button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                {/* Pagination */}
                {totalPages > 1 && (
                  <div className="mt-4">
                    <SimplePagination
                      currentPage={page}
                      totalPages={totalPages}
                      onPageChange={setPage}
                    />
                  </div>
                )}
              </>
            )}
          </CardContent>
        </Card>
      </main>

      {/* Create User Dialog */}
      <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>{t("users.newUser")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 px-6 py-4">
            <div>
              <label className="text-sm font-medium mb-2">{t("common.email")}</label>
              <Input
                type="email"
                value={formData.email}
                onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                placeholder={t("users.emailPlaceholder")}
              />
            </div>
            <div>
              <label className="text-sm font-medium mb-2">{t("users.name")}</label>
              <Input
                type="text"
                value={formData.name || ""}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder={t("users.namePlaceholder")}
              />
            </div>
            <div>
              <label className="text-sm font-medium mb-2">{t("users.password")}</label>
              <Input
                type="password"
                value={formData.password}
                onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                placeholder="••••••••"
              />
            </div>
            <div>
              <label className="text-sm font-medium mb-2">{t("users.roles")}</label>
              <div className="flex gap-2 flex-wrap">
                {ROLE_OPTIONS.map((role) => (
                  <button
                    key={role.value}
                    type="button"
                    onClick={() => toggleRole(role.value)}
                    className={`px-3 py-1.5 rounded-full text-sm transition-colors ${
                      formData.roles?.includes(role.value)
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted hover:bg-muted/80"
                    }`}
                  >
                    {role.label}
                  </button>
                ))}
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowCreateDialog(false)}>
              {t("common.cancel")}
            </Button>
            <Button className="bg-gradient-to-r from-brain-primary to-brain-accent hover:from-brain-primary-dark hover:to-brain-accent-dark text-white" onClick={handleCreateUser} disabled={!formData.email || !formData.password}>
              {t("users.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit User Dialog */}
      <Dialog open={showEditDialog} onOpenChange={setShowEditDialog}>
        <DialogContent className="sm:max-w-[500px]">
          <DialogHeader>
            <DialogTitle>{t("users.editUser")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 px-6 py-4">
            <div>
              <label className="text-sm font-medium mb-2">{t("common.email")}</label>
              <Input type="email" value={formData.email} disabled />
            </div>
            <div>
              <label className="text-sm font-medium mb-2">{t("users.name")}</label>
              <Input
                type="text"
                value={formData.name || ""}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                placeholder={t("users.namePlaceholder")}
              />
            </div>
            <div>
              <label className="text-sm font-medium mb-2">{t("users.columns.status")}</label>
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
            <div>
              <label className="text-sm font-medium mb-2">{t("users.roles")}</label>
              <div className="flex gap-2 flex-wrap">
                {ROLE_OPTIONS.map((role) => (
                  <button
                    key={role.value}
                    type="button"
                    onClick={() => toggleRole(role.value)}
                    className={`px-3 py-1.5 rounded-full text-sm transition-colors ${
                      formData.roles?.includes(role.value)
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted hover:bg-muted/80"
                    }`}
                  >
                    {role.label}
                  </button>
                ))}
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowEditDialog(false)}>
              {t("common.cancel")}
            </Button>
            <Button className="bg-gradient-to-r from-brain-primary to-brain-accent hover:from-brain-primary-dark hover:to-brain-accent-dark text-white" onClick={handleUpdateUser}>{t("users.saveChanges")}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
