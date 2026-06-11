Practical 重跑规则：`run_mode=1` 且 `start_from=align` 时，不传 `chapter_nums`/`block_nums` 会执行全量 align、无需已有 `script_aligned.json`，只有局部重跑时才要求已有 `script_aligned.json` 用于复用未重跑章节的对齐结果。
Practical 的 persist 阶段优先使用 `script_aligned.json`，只有在它不存在时才回退到 `script_input.json`。
Practical 单独重跑部分 `block_nums`/`chapter_nums` 的语音并成功跑到 `align` 后，也会重建并覆盖整个项目的 `dialogue.wav` 和 `script_aligned.json`；因此后续可以直接从 `images` 或 `render` 开始继续跑，不影响最终成片。
如果旧逻辑已经删掉了 `script_aligned.json`，但项目里仍保留全量 `block/chapter` 原始音频，那么局部重跑进入 `align` 时会自动改成一次全量 align 来恢复 `script_aligned.json`，不会重新请求其他章节的 TTS。
