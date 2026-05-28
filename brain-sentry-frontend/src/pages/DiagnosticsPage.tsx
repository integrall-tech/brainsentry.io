import { useEffect, useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import {
  Activity, RefreshCw, CheckCircle2, AlertTriangle, XCircle, MinusCircle,
} from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { EmptyState } from "@/components/ui/EmptyState";
import { api, type DiagnosticsReport, type DiagnosticsCheck, type DiagnosticsStatus } from "@/lib/api/client";

const STATUS_CLASSES: Record<DiagnosticsStatus, string> = {
  ok: "text-emerald-600 dark:text-emerald-400",
  warn: "text-amber-600 dark:text-amber-400",
  fail: "text-rose-600 dark:text-rose-400",
  skip: "text-muted-foreground",
};

const ICONS: Record<DiagnosticsStatus, typeof CheckCircle2> = {
  ok: CheckCircle2,
  warn: AlertTriangle,
  fail: XCircle,
  skip: MinusCircle,
};

export default function DiagnosticsPage() {
  const { t } = useTranslation();
  const [report, setReport] = useState<DiagnosticsReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const r = await api.getDiagnostics();
      setReport(r);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const aggregate = report?.status ?? "skip";
  const AggregateIcon = ICONS[aggregate];

  return (
    <div className="p-6 space-y-6" data-testid="diagnostics-page">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold flex items-center gap-2">
            <Activity className="h-6 w-6" /> {t("diagnostics.title")}
          </h1>
          <p className="text-sm text-muted-foreground mt-1 max-w-2xl">
            {t("diagnostics.subtitle")}
          </p>
        </div>
        <Button onClick={refresh} disabled={loading} variant="outline" size="sm" data-testid="diagnostics-refresh">
          <RefreshCw className={`h-4 w-4 mr-2 ${loading ? "animate-spin" : ""}`} />
          {t("diagnostics.refresh")}
        </Button>
      </header>

      {error && (
        <Card>
          <CardContent className="p-4">
            <p className="text-sm text-rose-600 dark:text-rose-400" data-testid="diagnostics-error">
              {error}
            </p>
          </CardContent>
        </Card>
      )}

      {report && (
        <Card data-testid="diagnostics-summary">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <AggregateIcon className={`h-5 w-5 ${STATUS_CLASSES[aggregate]}`} />
              <span data-testid="diagnostics-aggregate-status">
                {t(`diagnostics.status.${aggregate}`)}
              </span>
              <span className="text-xs text-muted-foreground ml-2">
                ({report.duration_ms}ms)
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-4 gap-4 text-sm">
              <SummaryStat label={t("diagnostics.summary.ok")} value={report.summary.ok} status="ok" />
              <SummaryStat label={t("diagnostics.summary.warn")} value={report.summary.warn} status="warn" />
              <SummaryStat label={t("diagnostics.summary.fail")} value={report.summary.fail} status="fail" />
              <SummaryStat label={t("diagnostics.summary.skip")} value={report.summary.skip} status="skip" />
            </div>
          </CardContent>
        </Card>
      )}

      {loading && !report && (
        <div className="flex items-center justify-center py-12">
          <Spinner />
        </div>
      )}

      {report && report.checks.length === 0 && !loading && (
        <EmptyState icon={Activity} title={t("diagnostics.empty")} description={t("diagnostics.empty")} />
      )}

      {report && report.checks.length > 0 && (
        <div className="space-y-3" data-testid="diagnostics-checks">
          {report.checks.map((c) => (
            <CheckRow key={c.name} check={c} />
          ))}
        </div>
      )}
    </div>
  );
}

function SummaryStat({ label, value, status }: { label: string; value: number; status: DiagnosticsStatus }) {
  return (
    <div className="text-center">
      <div className={`text-2xl font-semibold ${STATUS_CLASSES[status]}`} data-testid={`diagnostics-stat-${status}`}>
        {value}
      </div>
      <div className="text-xs text-muted-foreground uppercase tracking-wide">{label}</div>
    </div>
  );
}

function CheckRow({ check }: { check: DiagnosticsCheck }) {
  const Icon = ICONS[check.status];
  const cls = STATUS_CLASSES[check.status];
  return (
    <Card data-testid={`diagnostics-check-${check.name}`} data-status={check.status}>
      <CardContent className="p-4 flex gap-3">
        <Icon className={`h-5 w-5 mt-0.5 flex-shrink-0 ${cls}`} />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <span className="font-medium">{check.name}</span>
            <span className={`text-xs uppercase tracking-wide ${cls}`}>{check.status}</span>
            <span className="text-xs text-muted-foreground ml-auto">{check.duration_ms}ms</span>
          </div>
          <p className="text-sm mt-1">{check.message}</p>
          {check.detail && (
            <p className="text-xs text-muted-foreground mt-1 break-all">
              {check.detail}
            </p>
          )}
          {check.hint && (
            <p className="text-xs italic mt-1 text-muted-foreground">
              {check.hint}
            </p>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
