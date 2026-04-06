export default function LoadingPodcastScriptPage() {
  return (
    <main className="shell">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
        <section className="glass-panel p-8 md:p-10">
          <div className="h-5 w-40 animate-pulse rounded-full bg-muted" />
          <div className="mt-6 h-16 w-4/5 animate-pulse rounded-3xl bg-muted" />
          <div className="mt-5 h-5 w-3/5 animate-pulse rounded-full bg-muted" />
          <div className="mt-10 space-y-4">
            <div className="h-24 animate-pulse rounded-3xl bg-muted" />
            <div className="h-24 animate-pulse rounded-3xl bg-muted" />
            <div className="h-24 animate-pulse rounded-3xl bg-muted" />
          </div>
        </section>
        <aside className="space-y-6">
          <div className="glass-panel h-48 animate-pulse" />
          <div className="glass-panel h-72 animate-pulse" />
        </aside>
      </div>
    </main>
  );
}
