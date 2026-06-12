import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { Brain, LogIn, AlertCircle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui";
import { Input } from "@/components/ui/filter";
import { Spinner } from "@/components/ui/spinner";
import { LanguageSwitcher } from "@/components/ui/language-switcher";
import { useAuth } from "@/contexts/AuthContext";
import { useToast } from "@/components/ui/toast";
import { API_BASE_URL as API_URL } from "@/lib/api/client";

// O login demo (usuário compartilhado) só aparece em dev ou quando o build
// de homologação pede explicitamente via VITE_DEMO_LOGIN=true. O backend
// também precisa estar com security.demo_auth_enabled.
const DEMO_LOGIN_ENABLED =
  import.meta.env.DEV || import.meta.env.VITE_DEMO_LOGIN === "true";

export function LoginPage() {
  const { t } = useTranslation();
  const { login } = useAuth();
  const { toast } = useToast();
  const navigate = useNavigate();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setIsLoading(true);

    try {
      await login(email, password);
      toast({
        title: t("auth.loginSuccess"),
        description: t("auth.welcomeBack"),
        variant: "success",
      });
      navigate("/app/dashboard");
    } catch (err) {
      const errorMessage = (err as Error).message || t("auth.invalidCredentials");
      setError(errorMessage);
      toast({
        title: t("auth.loginError"),
        description: errorMessage,
        variant: "error",
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleDemoLogin = async () => {
    setEmail("demo@example.com");
    setPassword("demo123");
    setError("");
    setIsLoading(true);

    try {
      await fetch(`${API_URL}/v1/auth/demo`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
      });

      await login("demo@example.com", "demo123");

      toast({
        title: t("auth.demoSuccess"),
        description: t("auth.demoDesc"),
        variant: "success",
      });
      navigate("/app/dashboard");
    } catch {
      setError(t("auth.demoUnavailable"));
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-background to-muted flex items-center justify-center p-4">
      <div className="absolute top-4 right-4">
        <LanguageSwitcher />
      </div>
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center p-3 bg-primary/10 rounded-full mb-4">
            <Brain className="h-10 w-10 text-primary" />
          </div>
          <h1 className="text-3xl font-bold">Brain Sentry</h1>
          <p className="text-muted-foreground">
            {t("auth.subtitle")}
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>{t("auth.login")}</CardTitle>
          </CardHeader>
          <CardContent>
            <form onSubmit={handleSubmit} className="space-y-4">
              {error && (
                <div className="flex items-center gap-2 p-3 bg-destructive/10 text-destructive rounded-md">
                  <AlertCircle className="h-4 w-4" />
                  <span className="text-sm">{error}</span>
                </div>
              )}

              <div className="space-y-2">
                <label htmlFor="email" className="text-sm font-medium">
                  {t("auth.email")}
                </label>
                <Input
                  id="email"
                  type="email"
                  placeholder={t("auth.emailPlaceholder")}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  disabled={isLoading}
                />
              </div>

              <div className="space-y-2">
                <label htmlFor="password" className="text-sm font-medium">
                  {t("auth.password")}
                </label>
                <Input
                  id="password"
                  type="password"
                  placeholder={t("auth.passwordPlaceholder")}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  disabled={isLoading}
                />
              </div>

              <div className="text-right">
                <a
                  href="#"
                  className="text-sm text-primary hover:underline"
                  onClick={(e) => {
                    e.preventDefault();
                    toast({
                      title: t("auth.forgotTitle"),
                      description: t("auth.forgotDesc"),
                      variant: "info",
                    });
                  }}
                >
                  {t("auth.forgot")}
                </a>
              </div>

              <Button type="submit" className="w-full" disabled={isLoading}>
                {isLoading ? (
                  <>
                    <Spinner size="sm" label={t("auth.signingIn")} />
                  </>
                ) : (
                  <>
                    <LogIn className="h-4 w-4 mr-2" />
                    {t("auth.login")}
                  </>
                )}
              </Button>

              {DEMO_LOGIN_ENABLED && (
                <>
                  <div className="relative my-6">
                    <div className="absolute inset-0 flex items-center">
                      <span className="w-full border-t" />
                    </div>
                    <div className="relative flex justify-center text-xs uppercase">
                      <span className="bg-background px-2 text-muted-foreground">
                        {t("auth.or")}
                      </span>
                    </div>
                  </div>

                  <Button
                    type="button"
                    variant="outline"
                    className="w-full"
                    onClick={handleDemoLogin}
                    disabled={isLoading}
                  >
                    {t("auth.demoLogin")}
                  </Button>
                </>
              )}
            </form>
          </CardContent>
        </Card>

        <p className="text-center text-sm text-muted-foreground mt-6">
          {t("auth.noAccount")}{" "}
          <a
            href="#"
            className="text-primary hover:underline"
            onClick={(e) => {
              e.preventDefault();
              toast({
                title: t("auth.registerTitle"),
                description: t("auth.registerDesc"),
                variant: "info",
              });
            }}
          >
            {t("auth.requestAccess")}
          </a>
        </p>

        <div className="mt-8 p-4 bg-muted/20 rounded-lg">
          <p className="text-xs text-muted-foreground text-center">
            {t("auth.devHint")}
          </p>
        </div>
      </div>
    </div>
  );
}
