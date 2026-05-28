import { useEffect, useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Cpu, RefreshCw, Stethoscope, CheckCircle2, XCircle } from "lucide-react";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import {
  api,
  type ModelResolveResult,
  type ModelsDoctorReport,
  type ModelProbeResult,
} from "@/lib/api/client";

export default function ModelsPage() {
  const { t } = useTranslation();
  const [snapshot, setSnapshot] = useState<ModelResolveResult[]>([]);
  const [doctor, setDoctor] = useState<ModelsDoctorReport | null>(null);
  const [loading, setLoading] = useState(false);
  const [probing, setProbing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadSnapshot = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const r = await api.getModelsSnapshot();
      setSnapshot(r.snapshot);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  const runDoctor = useCallback(async () => {
    setProbing(true);
    try {
      const r = await api.getModelsDoctor();
      setDoctor(r);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setProbing(false);
    }
  }, []);

  useEffect(() => {
    loadSnapshot();
  }, [loadSnapshot]);

  return (
    <div className="p-6 space-y-6" data-testid="models-page">
      <header className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold flex items-center gap-2">
            <Cpu className="h-6 w-6" /> {t("models.title")}
          </h1>
          <p className="text-sm text-muted-foreground mt-1 max-w-2xl">
            {t("models.subtitle")}
          </p>
        </div>
        <div className="flex gap-2">
          <Button onClick={loadSnapshot} disabled={loading} variant="outline" size="sm" data-testid="models-refresh">
            <RefreshCw className={`h-4 w-4 mr-2 ${loading ? "animate-spin" : ""}`} />
            {t("models.refresh")}
          </Button>
          <Button onClick={runDoctor} disabled={probing} size="sm" data-testid="models-run-doctor">
            <Stethoscope className={`h-4 w-4 mr-2 ${probing ? "animate-spin" : ""}`} />
            {t("models.runDoctor")}
          </Button>
        </div>
      </header>

      {error && (
        <Card>
          <CardContent className="p-4">
            <p className="text-sm text-rose-600 dark:text-rose-400" data-testid="models-error">
              {error}
            </p>
          </CardContent>
        </Card>
      )}

      <Card data-testid="models-snapshot">
        <CardHeader>
          <CardTitle>{t("models.routing")}</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          {loading && snapshot.length === 0 ? (
            <div className="p-6 flex justify-center"><Spinner /></div>
          ) : (
            <table className="w-full text-sm">
              <thead className="bg-muted/40">
                <tr>
                  <th className="text-left p-3">{t("models.col.tier")}</th>
                  <th className="text-left p-3">{t("models.col.model")}</th>
                  <th className="text-left p-3">{t("models.col.source")}</th>
                </tr>
              </thead>
              <tbody>
                {snapshot.map((r) => (
                  <tr key={r.tier} className="border-t" data-testid={`models-row-${r.tier}`}>
                    <td className="p-3 font-medium">{r.tier}</td>
                    <td className="p-3 font-mono text-xs">
                      {r.model || (
                        <span className="text-rose-600">({t("models.unresolved")})</span>
                      )}
                    </td>
                    <td className="p-3 text-muted-foreground">{r.source}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </CardContent>
      </Card>

      {doctor && (
        <Card data-testid="models-doctor">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              {doctor.ok ? (
                <>
                  <CheckCircle2 className="h-5 w-5 text-emerald-600" />
                  <span data-testid="models-doctor-aggregate">{t("models.doctor.allPass")}</span>
                </>
              ) : (
                <>
                  <XCircle className="h-5 w-5 text-rose-600" />
                  <span data-testid="models-doctor-aggregate">{t("models.doctor.someFail")}</span>
                </>
              )}
              <span className="text-xs text-muted-foreground ml-2">({doctor.duration_ms}ms)</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {doctor.results.map((p) => (
              <ProbeRow key={p.tier} probe={p} />
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function ProbeRow({ probe }: { probe: ModelProbeResult }) {
  const Icon = probe.ok ? CheckCircle2 : XCircle;
  const cls = probe.ok ? "text-emerald-600" : "text-rose-600";
  return (
    <div className="flex gap-3 items-start" data-testid={`models-probe-${probe.tier}`} data-status={probe.ok ? "ok" : "fail"}>
      <Icon className={`h-5 w-5 mt-0.5 flex-shrink-0 ${cls}`} />
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-medium">{probe.tier}</span>
          <span className="font-mono text-xs text-muted-foreground">{probe.model}</span>
          <span className="text-xs text-muted-foreground ml-auto">{probe.duration_ms}ms</span>
        </div>
        {!probe.ok && (
          <>
            <p className={`text-sm mt-1 ${cls}`}>{probe.failure}</p>
            {probe.detail && <p className="text-xs text-muted-foreground mt-1 break-all">{probe.detail}</p>}
            {probe.hint && <p className="text-xs italic mt-1 text-muted-foreground">{probe.hint}</p>}
          </>
        )}
      </div>
    </div>
  );
}
