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
              播客公开页现在只走数据库，不再直接读取 `worker/outputs`。
            </h1>

            <p className="mt-6 max-w-3xl text-lg leading-8 text-muted-foreground">
              公开内容入口已经收敛为单个脚本详情页。视频、脚本、词汇、语法和右侧商品位都从 backend 的结构化数据返回。
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
                每条播客对应一个独立页面，顶部优先嵌入 YouTube，其次降级为视频地址。
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm uppercase tracking-[0.2em] text-primary">
                  Pipeline
                </CardTitle>
              </CardHeader>
              <CardContent className="text-sm leading-7 text-muted-foreground">
                worker 在上传后会追加数据库持久化任务，把脚本页数据和项目状态写入 PostgreSQL。
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm uppercase tracking-[0.2em] text-primary">
                  Commerce Sidebar
                </CardTitle>
              </CardHeader>
              <CardContent className="text-sm leading-7 text-muted-foreground">
                右侧商品位已经留好，但当前先保持为空，后续只需要给数据库补商品数据。
              </CardContent>
            </Card>
          </div>
        </div>
      </section>
    </main>
  );
}
