import Link from "next/link";

import { listProjects } from "@/lib/api";
import { formatUpdatedAt } from "@/lib/format";

export const revalidate = 60;

export default async function HomePage() {
  const projects = await listProjects();

  return (
    <main className="page-shell">
      <section className="hero-panel">
        <p className="eyebrow">Public Website Bootstrap</p>
        <h1>把你的 `conversation_minimal.json` 直接变成可访问的网站内容。</h1>
        <p className="hero-copy">
          这一版先走最稳妥的方案：保留现有 Go 后端，前端用 Next.js 做展示页，内容直接读取
          `worker/outputs/projects` 里的项目产物。
        </p>
        <div className="hero-stats">
          <div>
            <span>项目数量</span>
            <strong>{projects.length}</strong>
          </div>
          <div>
            <span>内容来源</span>
            <strong>conversation_minimal.json</strong>
          </div>
          <div>
            <span>渲染方式</span>
            <strong>Next.js App Router</strong>
          </div>
        </div>
      </section>

      <section className="section-block">
        <div className="section-header">
          <div>
            <p className="eyebrow">Projects</p>
            <h2>公开内容列表</h2>
          </div>
          <p className="section-copy">首屏先展示项目卡片，点进去后再看完整对话、视频和音频。</p>
        </div>

        {projects.length === 0 ? (
          <div className="empty-state">
            <h3>还没有可展示的项目</h3>
            <p>请先确认 `worker/outputs/projects/*/conversation_minimal.json` 已生成。</p>
          </div>
        ) : (
          <div className="project-grid">
            {projects.map((project) => (
              <article className="project-card" key={project.project_id}>
                <div className="project-card-top">
                  <span className="pill">{project.language || "unknown"}</span>
                  <span className="pill pill-muted">
                    {project.segment_count} segments
                  </span>
                </div>

                <div className="project-card-body">
                  <h3>{project.title}</h3>
                  <p>{project.project_id}</p>
                </div>

                <dl className="meta-grid">
                  <div>
                    <dt>面向语言</dt>
                    <dd>{project.audience_language || "未设置"}</dd>
                  </div>
                  <div>
                    <dt>轮次数</dt>
                    <dd>{project.turn_count}</dd>
                  </div>
                  <div>
                    <dt>更新时间</dt>
                    <dd>{formatUpdatedAt(project.updated_at)}</dd>
                  </div>
                </dl>

                <Link className="card-link" href={`/projects/${project.project_id}`}>
                  查看详情
                </Link>
              </article>
            ))}
          </div>
        )}
      </section>
    </main>
  );
}
