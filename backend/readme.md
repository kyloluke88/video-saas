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
  "tts_type": 1,
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
    - 可选传 `block_nums`，例如 `[5,6]`，只重跑指定 block 的 TTS/对齐，其他 block 直接复用历史结果。
    - 不传或传空数组时，行为与普通重试一致：整集重新走音频生成流程。
  - `2`：同一个 `project_id` 仅重做 compose（跳过 TTS/MFA）。

- `tts_type`
  - `1`：Google Gemini TTS（多说话人），字幕时间轴使用现有 MFA 对齐链路。
  - `2`：ElevenLabs Dialogue（with timestamps，建议 `ELEVENLABS_TTS_MODEL=eleven_v3`），直接使用 ElevenLabs 返回的字符级时间戳生成 segment/token/highlight 时间轴，不走 MFA。
  - `run_mode=0` 不传时默认 `1`。
  - `run_mode=1` 会复用历史 `request_payload.json` 中记录的 `tts_type`。

- `seed`
  - 前端无需传入。
  - backend 在 `run_mode=0` 时基于 `project_id` 自动生成并写入 `request_payload.json`。
  - `run_mode=1` 直接复用该项目历史 payload 里的 `seed`；老项目若无 `seed`，worker 会回退为 `project_id` 哈希。

- `script` 中的 segment 文本字段（适用于 `tts_type=2`）
  - `text`：字幕显示文本（建议保持干净，不带情绪标签）。
  - `speech_text`（或 `tts_text`）：可选，专门给 Eleven 朗读的文本，可包含情绪/动作标签（如 `[indecisive]`、`[laughs]`）；如果为空则回退到 `text`。

- `project_id`
  - `run_mode=1/2` 必传。
  - `run_mode=0` 可不传，后端会自动生成。

- `block_nums`
  - 仅 `run_mode=1` 有效。
  - 传 1-based block 序号数组，例如 `[1, 3, 7]`。
  - 也兼容旧字段名 `block_num`。

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
  "tts_type": 2,
  "run_mode": 0,
  "script_filename": "zh_podcast_demo.json",
  "bg_img_filenames": ["podcast_bg_01.png"]
}
```

### `run_mode=1` 原项目重跑

```json
{
  "project_id": "zh_podcast_20260325120000_topic-demo",
  "run_mode": 1,
  "block_nums": [5, 6]
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
