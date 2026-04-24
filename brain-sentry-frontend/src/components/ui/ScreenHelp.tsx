import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import {
  BookOpen, Target, AlertCircle, ListChecks, Route, Ruler, X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { helpContent, getHelpContent, type HelpStep } from "@/lib/help/helpContent";

interface Props {
  open: boolean;
  onClose: () => void;
  route: string;
}

/**
 * ScreenHelp renders a right-side drawer with business-focused guidance
 * for the current screen. Content lives in src/lib/help/helpContent.ts
 * keyed by route path. Bilingual: picks pt-BR or en by active i18n language.
 */
export function ScreenHelp({ open, onClose, route }: Props) {
  const { t, i18n } = useTranslation();
  const lang = (i18n.language || "pt-BR").startsWith("pt") ? "ptBR" : "en";

  useEffect(() => {
    if (!open) return;
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handleEscape);
    return () => document.removeEventListener("keydown", handleEscape);
  }, [open, onClose]);

  useEffect(() => {
    if (open) document.body.style.overflow = "hidden";
    else document.body.style.overflow = "";
    return () => {
      document.body.style.overflow = "";
    };
  }, [open]);

  if (!open) return null;

  const entry = getHelpContent(route);
  const localized = entry?.[lang] ?? entry?.ptBR ?? helpContent["/app/dashboard"].ptBR;

  return (
    <div className="fixed inset-0 z-50 flex">
      <div
        data-testid="screen-help-overlay"
        className="fixed inset-0 bg-black/40 backdrop-blur-sm"
        onClick={onClose}
      />
      <aside
        role="dialog"
        aria-modal="true"
        aria-label={localized.titulo}
        data-testid="screen-help-drawer"
        className={cn(
          "ml-auto relative z-10 h-full w-full sm:w-[420px] md:w-[460px] bg-background border-l shadow-2xl flex flex-col",
          "animate-in slide-in-from-right duration-200",
        )}
      >
        <header className="px-4 py-4 border-b bg-gradient-to-r from-brain-primary to-brain-accent text-white">
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-center gap-2 min-w-0">
              <div className="p-1.5 bg-white/20 rounded-lg backdrop-blur-sm flex-shrink-0">
                <BookOpen className="h-4 w-4 text-white" />
              </div>
              <div className="min-w-0">
                <h2 className="text-sm font-bold leading-tight truncate">
                  {localized.titulo}
                </h2>
                <p className="text-[11px] text-white/80 truncate">
                  {localized.subtitulo}
                </p>
              </div>
            </div>
            <button
              onClick={onClose}
              aria-label={t("help.close")}
              className="p-1 rounded hover:bg-white/20 flex-shrink-0"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </header>

        <div className="flex-1 overflow-y-auto px-4 py-4 space-y-5 text-sm">
          <Section icon={Target} label={t("help.section.objetivo")}>
            <p className="text-foreground/90 leading-relaxed">{localized.objetivo}</p>
          </Section>

          <Section icon={AlertCircle} label={t("help.section.problema")}>
            <p className="text-foreground/80 leading-relaxed">{localized.problema}</p>
          </Section>

          <Section icon={ListChecks} label={t("help.section.comoFunciona")}>
            <ul className="space-y-1.5 list-none">
              {localized.comoFunciona.map((item, i) => (
                <li key={i} className="flex gap-2 leading-relaxed">
                  <span className="text-brain-primary mt-0.5">•</span>
                  <span>{item}</span>
                </li>
              ))}
            </ul>
          </Section>

          <Section icon={Route} label={t("help.section.fluxo")}>
            <ol className="space-y-2.5 list-none">
              {localized.fluxoSugerido.map((step: HelpStep) => (
                <li key={step.numero} className="flex gap-2.5">
                  <span className="flex-shrink-0 w-5 h-5 rounded-full bg-brain-primary/15 text-brain-primary text-[11px] font-bold flex items-center justify-center mt-0.5">
                    {step.numero}
                  </span>
                  <div className="min-w-0">
                    <p className="font-medium">{step.acao}</p>
                    {step.esperado && (
                      <p className="text-[12px] text-muted-foreground mt-0.5">
                        → {step.esperado}
                      </p>
                    )}
                  </div>
                </li>
              ))}
            </ol>
          </Section>

          {localized.regrasChave && localized.regrasChave.length > 0 && (
            <Section icon={Ruler} label={t("help.section.regras")}>
              <ul className="space-y-1.5 list-none">
                {localized.regrasChave.map((rule, i) => (
                  <li key={i} className="flex gap-2 leading-relaxed">
                    <span className="text-amber-500 mt-0.5">•</span>
                    <span>{rule}</span>
                  </li>
                ))}
              </ul>
            </Section>
          )}

          <p className="text-[11px] text-muted-foreground pt-2 border-t">
            {t("help.footer")}
          </p>
        </div>
      </aside>
    </div>
  );
}

function Section({
  icon: Icon,
  label,
  children,
}: {
  icon: typeof BookOpen;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <div className="flex items-center gap-1.5 mb-2 text-[11px] uppercase tracking-wider text-muted-foreground font-semibold">
        <Icon className="h-3.5 w-3.5" />
        {label}
      </div>
      <div className="pl-1">{children}</div>
    </section>
  );
}
