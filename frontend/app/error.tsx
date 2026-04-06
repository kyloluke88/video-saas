"use client";

import Link from "next/link";

import { Button } from "@/components/ui/button";

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-5xl items-center px-4 py-16">
      <section className="w-full rounded-3xl border border-border/60 bg-card/85 p-10 shadow-glow backdrop-blur">
        <p className="text-sm font-semibold uppercase tracking-[0.3em] text-primary">Application Error</p>
        <h1 className="mt-4 font-display text-5xl font-bold tracking-tight text-balance">
          页面暂时加载失败
        </h1>
        <p className="mt-4 max-w-2xl text-base leading-7 text-muted-foreground">
          {error?.message || "请稍后再试。"}
        </p>
        <div className="mt-8 flex flex-wrap gap-3">
          <Button onClick={() => reset()} type="button">
            重试
          </Button>
          <Button asChild variant="outline">
            <Link href="/">返回首页</Link>
          </Button>
        </div>
      </section>
    </main>
  );
}
