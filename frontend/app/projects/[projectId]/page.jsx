import Link from "next/link";
import { notFound } from "next/navigation";

import ConversationView from "@/components/conversation-view";
import { buildPublicAssetUrl, getProject } from "@/lib/api";
import { formatUpdatedAt } from "@/lib/format";

export const revalidate = 60;

export async function generateMetadata({ params }) {
  const project = await getProject(params.projectId);

  if (!project) {
    return {
      title: "Project Not Found",
    };
  }

  return {
    title: `${project.title} | Video SaaS Showcase`,
    description: `Conversation project ${project.project_id}`,
  };
}

export default async function ProjectDetailPage({ params }) {
  const project = await getProject(params.projectId);

  if (!project) {
    notFound();
  }

  const videoUrl = buildPublicAssetUrl(project.assets?.video_path);
  const audioUrl = buildPublicAssetUrl(project.assets?.audio_path);
  const subtitleUrl = buildPublicAssetUrl(project.assets?.subtitle_path);

  return (
    <main className="page-shell">
      <section className="detail-hero">
        <Link href="/" className="back-link">
          返回项目列表
        </Link>

        <div className="detail-heading">
          <div>
            <p className="eyebrow">Project Detail</p>
            <h1>{project.title}</h1>
            <p className="section-copy">{project.project_id}</p>
          </div>

          <div className="detail-metadata">
            <div>
              <span>语言</span>
              <strong>{project.language || "unknown"}</strong>
            </div>
            <div>
              <span>面向语言</span>
              <strong>{project.audience_language || "未设置"}</strong>
            </div>
            <div>
              <span>更新时间</span>
              <strong>{formatUpdatedAt(project.updated_at)}</strong>
            </div>
          </div>
        </div>
      </section>

      {(videoUrl || audioUrl) && (
        <section className="section-block media-panel">
          <div className="section-header">
            <div>
              <p className="eyebrow">Media</p>
              <h2>项目媒体资源</h2>
            </div>
            <p className="section-copy">后端直接从项目目录里读取公开媒体文件，前端这里按需展示。</p>
          </div>

          <div className="media-grid">
            {videoUrl ? (
              <div className="media-card">
                <h3>视频预览</h3>
                <video className="media-player" controls preload="metadata" src={videoUrl}>
                  {subtitleUrl ? (
                    <track default kind="subtitles" label="Subtitles" src={subtitleUrl} />
                  ) : null}
                </video>
              </div>
            ) : null}

            {audioUrl ? (
              <div className="media-card">
                <h3>音频预览</h3>
                <audio className="audio-player" controls preload="metadata" src={audioUrl} />
              </div>
            ) : null}
          </div>
        </section>
      )}

      <section className="section-block">
        <div className="section-header">
          <div>
            <p className="eyebrow">Conversation</p>
            <h2>对话内容</h2>
          </div>
          <p className="section-copy">
            基于 `conversation_minimal.json` 渲染，保留说话人、原文、英文辅助和注音提示。
          </p>
        </div>

        <ConversationView conversation={project.conversation} />
      </section>
    </main>
  );
}
