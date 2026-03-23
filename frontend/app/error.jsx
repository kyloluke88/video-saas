"use client";

import Link from "next/link";

export default function GlobalError({ error, reset }) {
  return (
    <main className="page-shell">
      <section className="empty-state">
        <p className="eyebrow">Application Error</p>
        <h1>页面暂时加载失败</h1>
        <p>{error?.message || "请稍后再试。"}</p>
        <div className="action-row">
          <button className="action-button" onClick={() => reset()} type="button">
            重试
          </button>
          <Link className="card-link" href="/">
            返回首页
          </Link>
        </div>
      </section>
    </main>
  );
}
