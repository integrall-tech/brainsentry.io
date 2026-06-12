import { useState } from "react";
import { Mail, Github } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useLanguage } from "../../contexts/LanguageContext";

export function CTASection() {
  const [email, setEmail] = useState("");
  const [submitted, setSubmitted] = useState(false);
  const { t } = useLanguage();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setEmail("");
    setSubmitted(true);
  };

  return (
    <section className="py-24 bg-gradient-to-b from-brain-primary/10 to-brain-accent/10 dark:from-brain-primary/20 dark:to-brain-accent/20" id="cta">
      <div className="container mx-auto px-4 sm:px-6 lg:px-8">
        <div className="max-w-2xl mx-auto text-center">
          <h2 className="text-4xl font-bold mb-4 dark:text-white">
            {t("cta.title")}
          </h2>
          <p className="text-xl text-muted-foreground dark:text-gray-300 mb-8">
            {t("cta.subtitle")}
          </p>

          {/* Email Form */}
          {submitted && (
            <p className="text-brain-success mb-4" role="status">
              {t("cta.thanks")}
            </p>
          )}
          <form onSubmit={handleSubmit} className="flex flex-col sm:flex-row gap-3 mb-8">
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t("cta.placeholder")}
              required
              className="flex-1 px-4 py-3 rounded-lg border-2 border-border dark:border-border/60 bg-background dark:bg-background/80 focus:outline-none focus:ring-2 focus:ring-brain-primary dark:focus:ring-brain-primary/60 dark:text-white"
            />
            <Button
              type="submit"
              size="lg"
              className="h-12 px-8 bg-brain-gold hover:bg-brain-gold/90"
            >
              {t("cta.button")} →
            </Button>
          </form>

          {/* Benefits */}
          <div className="flex flex-wrap justify-center gap-4 mb-8 text-sm">
            <div className="flex items-center gap-2 text-muted-foreground dark:text-gray-300">
              <span className="text-brain-success">✓</span>
              {t("cta.benefit1")}
            </div>
            <div className="flex items-center gap-2 text-muted-foreground dark:text-gray-300">
              <span className="text-brain-success">✓</span>
              {t("cta.benefit2")}
            </div>
            <div className="flex items-center gap-2 text-muted-foreground dark:text-gray-300">
              <span className="text-brain-success">✓</span>
              {t("cta.benefit3")}
            </div>
            <div className="flex items-center gap-2 text-muted-foreground dark:text-gray-300">
              <span className="text-brain-success">✓</span>
              {t("cta.benefit4")}
            </div>
          </div>

          {/* GitHub CTA */}
          <div className="text-center">
            <p className="text-sm text-muted-foreground dark:text-gray-400 mb-4">{t("cta.or")}</p>
            <a
              href="https://github.com/edsonmartins/brainsentry.io"
              target="_blank"
              rel="noopener noreferrer"
            >
              <Button variant="outline" size="lg" className="h-12 px-8 dark:border-border/60 dark:hover:bg-muted/20">
                <Github className="w-4 h-4 mr-2" />
                {t("cta.github")}
              </Button>
            </a>
          </div>
        </div>
      </div>
    </section>
  );
}
