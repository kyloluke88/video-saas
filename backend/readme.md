## Podcast 接口说明

### 创建接口

```text
POST /api/video/content/podcast/create
```

请求示例：

```json
{
  "project_id": "zh_podcast_20260325120000_topic-demo",
  "title": "Why Young Chinese Are Not Getting Married",
  "lang": "zh",
  "content_profile": "international",
  "run_mode": 0,
  "script_filename": "chinese_podcast_stage2_complete.json",
  "bg_img_filenames": ["podcast_bg_01.png", "podcast_bg_02.png"],
  "target_platform": "youtube",
  "aspect_ratio": "16:9",
  "resolution": "1080p",
  "design_style": 2
}
```

## 字段规则

- `run_mode`
  - `0`：新项目，全流程（脚本读取/TTS/MFA/合成）。
  - `1`：同一个 `project_id` 原地重跑（复用 `request_payload.json`）。
  - `2`：同一个 `project_id` 仅重做 compose（跳过 TTS/MFA）。

- `project_id`
  - `run_mode=1/2` 必传。
  - `run_mode=0` 可不传，后端会自动生成。

- `lang`
  - 可选值：`zh`、`ja`。
  - `run_mode=0` 必传。

- `content_profile`
  - 可选值：`daily`、`social_issue`、`international`。
  - `run_mode=0` 必传。

- `script_filename`
  - `run_mode=0` 必传。

- `bg_img_filenames`
  - 背景图数组，`run_mode=0` 必传。
  - 现在所有 `design_style` 都是静态背景图。
  - 仅使用数组第 1 张图作为背景；后续元素会被忽略（用于兼容旧请求结构）。
  - 不再存在 chapter 数量匹配校验。

- `design_style`
  - 可选值：`1`、`2`、`3`。
  - 不再控制背景动效。
  - 仅作为字幕 preset 的选择参数（保留分支结构，便于后续扩展字幕风格）。

- `resolution`
  - 可选值：`480p`、`720p`、`1080p`、`1440p`、`2000p`。
  - `run_mode=2` 可用于快速重做导出分辨率。

## run_mode 最小请求

### `run_mode=0` 新项目

```json
{
  "lang": "zh",
  "content_profile": "daily",
  "run_mode": 0,
  "script_filename": "zh_podcast_demo.json",
  "bg_img_filenames": ["podcast_bg_01.png"]
}
```

### `run_mode=1` 原项目重跑

```json
{
  "project_id": "zh_podcast_20260325120000_topic-demo",
  "run_mode": 1
}
```

### `run_mode=2` 仅重做合成

```json
{
  "project_id": "zh_podcast_20260325120000_topic-demo",
  "run_mode": 2,
  "bg_img_filenames": ["podcast_bg_01.png"],
  "resolution": "1080p",
  "design_style": 2
}
```
