import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export const dynamic = "force-dynamic";

export default function HomePage() {
  return (
    <main className="shell">
      <section className="glass-panel overflow-hidden p-8 md:p-12">
        <div className="grid gap-8 lg:grid-cols-[1.15fr_0.85fr] lg:items-end">
          <div>
            <div className="flex flex-wrap gap-2">
              <Badge>Podcast Scripts</Badge>
              <Badge variant="secondary">Database Driven</Badge>
            </div>

            <h1 className="mt-4 max-w-4xl font-display text-5xl font-bold leading-none tracking-tight text-balance md:text-7xl">
              The public pages are now fully database-driven.
            </h1>

            <p className="mt-6 max-w-3xl text-lg leading-8 text-muted-foreground">
              Scripts, product catalog pages, and product detail pages now pull structured data from backend APIs instead of reading static worker outputs.
            </p>
          </div>

          <div className="grid gap-4">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm uppercase tracking-[0.2em] text-primary">
                  Script Page
                </CardTitle>
              </CardHeader>
              <CardContent className="text-sm leading-7 text-muted-foreground">
                Each podcast has a dedicated detail page. YouTube is prioritized, with a fallback to direct video URLs.
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm uppercase tracking-[0.2em] text-primary">
                  Pipeline
                </CardTitle>
              </CardHeader>
              <CardContent className="text-sm leading-7 text-muted-foreground">
                The generation pipeline persists script and project data to PostgreSQL for stable public rendering.
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm uppercase tracking-[0.2em] text-primary">
                  Commerce Sidebar
                </CardTitle>
              </CardHeader>
              <CardContent className="text-sm leading-7 text-muted-foreground">
                Product list/detail routes are now available with localized catalogs and podcast recommendations.
              </CardContent>
            </Card>
          </div>
        </div>
      </section>
    </main>
  );
}
