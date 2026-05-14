import { Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider, useAuth } from "./contexts/AuthContext";
import { ThemeProvider } from "./contexts/ThemeContext";
import { ToastProvider, ToastProvider as ToastProviderComp } from "./components/ui/toast";
import { ErrorBoundary } from "./components/ui/error-boundary";
import { AdminLayout } from "./components/layout/AdminLayout";
import { LoginPage } from "./pages/LoginPage";
import { DashboardPage } from "./pages/DashboardPage";
import { SearchPage } from "./pages/SearchPage";
import { RelationshipsPage } from "./pages/RelationshipsPage";
import { AuditPage } from "./pages/AuditPage";
import { ConfigurationPage } from "./pages/ConfigurationPage";
import { UsersPage } from "./pages/UsersPage";
import { TenantsPage } from "./pages/TenantsPage";
import MemoryAdminPage from "./pages/MemoryAdminPage";
import AnalyticsAdminPage from "./pages/AnalyticsAdminPage";
import ProfilePage from "./pages/ProfilePage";
import PlaygroundPage from "./pages/PlaygroundPage";
import ConnectorsPage from "./pages/ConnectorsPage";
import NotesPage from "./pages/NotesPage";
import TasksPage from "./pages/TasksPage";
import TimelinePage from "./pages/TimelinePage";
import ConsolePage from "./pages/ConsolePage";
import AgentTracesPage from "./pages/AgentTracesPage";
import ExtractionLabPage from "./pages/ExtractionLabPage";
import OntologyPage from "./pages/OntologyPage";
import SessionCachePage from "./pages/SessionCachePage";
import ActionsPage from "./pages/ActionsPage";
import MeshPage from "./pages/MeshPage";
import BatchSearchPage from "./pages/BatchSearchPage";
import DecisionsPage from "./pages/DecisionsPage";
import PoliciesPage from "./pages/PoliciesPage";
import EventsPage from "./pages/EventsPage";
import ReasoningPage from "./pages/ReasoningPage";
import ProvenancePage from "./pages/ProvenancePage";
import GraphGlobalPage from "./pages/GraphGlobalPage";
import GraphEgoPage from "./pages/GraphEgoPage";
import GraphTimelinePage from "./pages/GraphTimelinePage";
import DiagnosticsPage from "./pages/DiagnosticsPage";
import { LandingPage } from "./landing/pages/LandingPage";

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <div className="inline-block h-8 w-8 animate-spin rounded-full border-4 border-primary border-t-transparent" />
          <p className="mt-4 text-muted-foreground">Carregando...</p>
        </div>
      </div>
    );
  }

  return isAuthenticated ? <>{children}</> : <Navigate to="/login" replace />;
}

function App() {
  return (
    <ErrorBoundary>
      <ThemeProvider>
        <ToastProviderComp>
          <AuthProvider>
            <Routes>
              {/* Landing Page - Public */}
              <Route path="/" element={<LandingPage />} />

              {/* Login Page */}
              <Route path="/login" element={<LoginPage />} />

              {/* Protected Routes - App */}
              <Route
                path="/app"
                element={
                  <ProtectedRoute>
                    <AdminLayout />
                  </ProtectedRoute>
                }
              >
                <Route index element={<Navigate to="/app/dashboard" replace />} />
                <Route path="dashboard" element={<DashboardPage />} />
                <Route path="memories" element={<MemoryAdminPage />} />
                <Route path="search" element={<SearchPage />} />
                <Route path="relationships" element={<RelationshipsPage />} />
                <Route path="audit" element={<AuditPage />} />
                <Route path="configuration" element={<ConfigurationPage />} />
                <Route path="users" element={<UsersPage />} />
                <Route path="tenants" element={<TenantsPage />} />
                <Route path="analytics" element={<AnalyticsAdminPage />} />
                <Route path="profile" element={<ProfilePage />} />
                <Route path="playground" element={<PlaygroundPage />} />
                <Route path="connectors" element={<ConnectorsPage />} />
                <Route path="notes" element={<NotesPage />} />
                <Route path="tasks" element={<TasksPage />} />
                <Route path="timeline" element={<TimelinePage />} />
                <Route path="console" element={<ConsolePage />} />
                <Route path="traces" element={<AgentTracesPage />} />
                <Route path="extraction" element={<ExtractionLabPage />} />
                <Route path="ontology" element={<OntologyPage />} />
                <Route path="session-cache" element={<SessionCachePage />} />
                <Route path="actions" element={<ActionsPage />} />
                <Route path="mesh" element={<MeshPage />} />
                <Route path="batch-search" element={<BatchSearchPage />} />
                <Route path="decisions" element={<DecisionsPage />} />
                <Route path="policies" element={<PoliciesPage />} />
                <Route path="events" element={<EventsPage />} />
                <Route path="reasoning" element={<ReasoningPage />} />
                <Route path="provenance" element={<ProvenancePage />} />
                <Route path="graph/global" element={<GraphGlobalPage />} />
                <Route path="graph/ego" element={<GraphEgoPage />} />
                <Route path="graph/timeline" element={<GraphTimelinePage />} />
                <Route path="diagnostics" element={<DiagnosticsPage />} />
              </Route>

              {/* Legacy redirect - /dashboard -> /app/dashboard */}
              <Route path="/dashboard" element={<Navigate to="/app/dashboard" replace />} />

              {/* Catch all - redirect to landing */}
              <Route path="*" element={<Navigate to="/" replace />} />
            </Routes>
          </AuthProvider>
        </ToastProviderComp>
      </ThemeProvider>
    </ErrorBoundary>
  );
}

export default App;
