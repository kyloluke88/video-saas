import Link from "next/link";

import { Button } from "@/components/ui/button";

export default function NotFoundPage() {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-5xl items-center px-4 py-16">
      <section className="w-full rounded-3xl border border-border/60 bg-card/85 p-10 shadow-glow backdrop-blur">
        <p className="text-sm font-semibold uppercase tracking-[0.3em] text-primary">404</p>
        <h1 className="mt-4 font-display text-5xl font-bold tracking-tight">内容不存在</h1>
        <p className="mt-4 max-w-2xl text-base leading-7 text-muted-foreground">
          请确认对应的公开内容已经发布，或者检查 URL 是否正确。
        </p>
        <div className="mt-8">
          <Button asChild>
            <Link href="/">返回首页</Link>
          </Button>
        </div>
      </section>
    </main>
  );
}
