## 前端传递规则

### Podcast 创建接口

接口：

```text
POST /api/video/content/podcast/create
```

当前请求结构：

```json
{
  "project_id": "zh_podcast_20260321160958_chinese-podcast-young-china-relationship",
  "title": "Why Young Chinese Are Not Getting Married",
  "lang": "zh",
  "content_profile": "daily",
  "run_mode": 0,
  "script_filename": "zh_podcast_young_china_relationship.json",
  "bg_img_filename": "podcast_bg_01.png",
  "target_platform": "youtube",
  "aspect_ratio": "16:9",
  "resolution": "1080p",
  "design_style": 1
}
```

字段说明：

- `project_id`
  - `run_mode=1` 或 `run_mode=2` 时必传。
  - `run_mode=0` 时可不传，后端会自动生成新的 `project_id`。

- `title`
  - 选填。
  - `run_mode=0` 时会参与新项目 `project_id` 的 slug 生成。

- `lang`
  - 可选值：`zh`、`ja`
  - `run_mode=0` 时必传。
  - `run_mode=1` / `run_mode=2` 时一般沿用旧项目，通常可不传。

- `content_profile`
  - 可选值：`daily`、`social_issue`、`international`
  - `run_mode=0` 时必传。

- `run_mode`
  - 可选值：`0`、`1`、`2`
  - 不传时默认按 `0` 处理。

- `script_filename`
  - `run_mode=0` 时必传。
  - `run_mode=1` / `run_mode=2` 时通常不需要传，直接复用旧项目。

- `bg_img_filename`
  - `run_mode=0` 时必传。
  - `run_mode=2` 时建议传，用来快速测试新的背景图或包装效果。

- `target_platform`
  - 选填，可选值：`youtube`、`tiktok`

- `aspect_ratio`
  - 选填。

- `resolution`
  - 选填，可选值：`480p`、`720p`、`1080p`、`1440p`、`2000p`
  - `run_mode=2` 时可传新值，用来快速重做导出分辨率。

- `design_style`
  - 选填，可选值：`1`、`2`、`3`
  - 不传时默认按 `1` 处理。
  - 当前只影响背景动效 preset，字幕和波形样式暂时保持一致。

### run_mode 说明

#### run_mode = 0

正常新项目生成。

行为：

- 自动生成新的 `project_id`
- 正常跑脚本读取、TTS、MFA 对齐、字幕高亮、最终合成

最少应传：

```json
{
  "lang": "zh",
  "content_profile": "daily",
  "run_mode": 0,
  "script_filename": "zh_podcast_demo.json",
  "bg_img_filename": "podcast_bg_01.png"
}
```

#### run_mode = 1

指定已有 `project_id`，按旧项目的 `request_payload.json` 原项目重跑。

行为：

- 读取旧项目目录下保存的 `request_payload.json`
- 继续走完整的音频生成和对齐流程
- 适合项目中途失败后继续跑

注意：

- 这是“原项目原地续跑/重跑”
- 不会新建项目
- 也不会优先使用这次请求里新传的视觉参数

最少应传：

```json
{
  "project_id": "zh_podcast_20260321160958_chinese-podcast-young-china-relationship",
  "run_mode": 1
}
```

#### run_mode = 2

指定已有 `project_id`，直接从现有 `dialogue.mp3 + script_aligned.json` 开始重做 compose。

行为：

- 跳过 TTS
- 跳过 MFA 对齐
- 跳过字幕高亮重新计算
- 直接进入视频包装层重新合成

适合：

- 快速测试背景图
- 快速测试 `design_style`
- 快速测试导出分辨率
- 不想重复跑音频和对齐

注意：

- 当前是“原项目原地重做包装层”
- 会覆盖原项目下的 `podcast_final.mp4`
- 适合反复测试视觉版本

最少应传：

```json
{
  "project_id": "zh_podcast_20260321160958_chinese-podcast-young-china-relationship",
  "run_mode": 2,
  "bg_img_filename": "podcast_bg_02.png",
  "resolution": "1080p",
  "design_style": 3
}
```

### design_style 说明

`design_style` 继续使用整数枚举，不传字符串。

当前映射：

- `1 = Calm Drift`
  - 轻缩放 + 轻横移

- `2 = Soft Parallax`
  - 双层背景视差

- `3 = Study Glow`
  - 轻渐变光影/学习氛围感背景

当前约定：

- `design_style` 只控制背景动效 preset
- 字幕 preset 结构已保留 `1/2/3` 分支
- 当前 `design_style=1/2/3` 暂时统一使用原先 `design_style=2` 的那套字幕 preset，便于继续沿用已有字幕调试基线
- 波形 preset 结构也已保留 `1/2/3` 分支，但当前三种 style 暂时共用同一套波形样式

这样后续如果要让不同 `design_style` 使用不同字幕 preset 或波形 preset，可以直接在现有分支上扩展，不需要再改接口结构。
