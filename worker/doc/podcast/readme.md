Podcast 规则：

1. `persist` 阶段只使用 `script_aligned.json`，没有 aligned 脚本就不会回退到 `script_input.json`。
2. 单独重跑某些 `block_nums` 的语音并成功跑到 `align` 后，会重建并覆盖整个项目的 `dialogue.mp3` 和 `script_aligned.json`；因此后续可以直接从 `render` 开始继续跑，不影响最终成片。
3. `start_from` 表示本次从哪个阶段开始，`stop_at` 表示本次在哪个阶段停止；`run_mode=1` 会先继承项目里旧的 `request_payload.json`，所以从更晚阶段重跑时要显式传入 `stop_at`，并且 `stop_at` 必须不早于 `start_from`。
