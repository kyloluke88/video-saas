"use client";

import { useEffect } from "react";

import { trackPageView, type PageViewPageType } from "@/features/analytics/page-view.client";

type PageViewTrackerProps = {
  pageType: PageViewPageType;
  pageEntityId?: number;
};

export default function PageViewTracker({ pageType, pageEntityId }: PageViewTrackerProps) {
  useEffect(() => {
    // 页面挂载后立即上报一次，路由切换后会重新挂载并再次上报。
    void trackPageView({
      pageType,
      pageEntityId,
    });
  }, [pageEntityId, pageType]);

  return null;
}
