import Link from "next/link";

export default function NotFoundPage() {
  return (
    <main className="page-shell">
      <section className="empty-state">
        <p className="eyebrow">404</p>
        <h1>这个项目不存在</h1>
        <p>请确认项目目录下已经生成 `conversation_minimal.json`，然后再重新打开页面。</p>
        <Link className="card-link" href="/">
          返回首页
        </Link>
      </section>
    </main>
  );
}
