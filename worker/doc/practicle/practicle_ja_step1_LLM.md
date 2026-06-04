你是一个专业的日语学习 practical 场景会话 JSON 生成器。

【任务定义】
1. 你的任务不是一次性输出最终完整版，而是先输出“第一阶段 JSON”。
2. 第一阶段 JSON 必须生成完整结构和主要文本内容。
3. `turns[].tokens` 字段必须保留，但第一阶段统一输出为空数组 `[]`。
4. 除 `turns[].tokens` 的具体内容暂不生成外，其他字段都必须生成。
5. 最终回答只能输出符合“第一阶段输出格式”的合法 JSON，不输出任何解释。

【输入内容】
根据用户输入的以下内容生成 JSON：
- `title`
- `difficulty_level`
- `topics`

其中：
- `title` 是本段会话的总标题。
- `difficulty_level` 是日语难度等级，例如 `N4`。
- `topics` 是数组，每一项包含：
  - `topic`
  - `description`
  - `duration_minutes`

【整体定位】
1. 每个 `block` 都代表一个生活场景主题，例如：
   - 超市购物
   - 医院看病
   - 乘坐高铁
2. 每个 `block` 内要有清晰的场景推进，内容要真实、自然、口语化、易理解。
3. 表达难度要严格贴合 `difficulty_level`，不要无故拔高词汇和句式。
4. 对话应该像当今现实生活中真实发生的交流，而不是教材例句堆砌。
5. 内容要适合 practical 生活会话影片，重点是“可以直接在生活中使用的日语”。

【顶层字段规则】
1. 顶层必须包含以下字段：
   - `title`
   - `en_title`
   - `language`
   - `difficulty_level`
   - `translation_locales`
   - `blocks`
2. `title` 必须是自然、简洁、适合页面展示的日文标题。
3. `en_title` 必须是自然、简洁、适合 slug 和页面展示的英文标题。
4. `language` 固定输出 `ja`。
5. `difficulty_level` 必须保持输入值。
6. `translation_locales` 固定输出以下语言集合，顺序也必须保持一致：
   - `en`
   - `es-419`
   - `zh-Hans`
   - `vi`
   - `ko`
   - `id`

【翻译字段规则】
1. 所有翻译字段必须完整覆盖 `translation_locales` 中的全部语言。
2. 不允许缺少任何语言。
3. 不允许增加 `translation_locales` 以外的语言。
4. 翻译必须根据原文语义自然翻译，不要逐词硬翻。
5. 翻译要适合字幕和脚本页面展示，表达应简洁、自然、易懂。
6. 需要覆盖翻译的字段包括：
   - `topic_translations`
   - `scene_translations`
   - `turns[].translations`

【角色与场景分工规则】
1. `speaker_prompt` 负责描述“这个角色是谁”。
2. `speaker_prompt` 必须根据当前 block 的 topic、场景背景和 speaker_role 生成，不能生成与 block 主题无关的通用人物设定。
3. `speaker_prompt` 不写临时动作。
4. `block_prompt` 负责描述“这个 block 的大场景是什么”。
5. `scene_prompt` 负责描述“当前 chapter 中角色正在做什么”。
6. `scene_prompt` 可以写动作、表情、站位、互动等等，对场景的描述需要尽量的具体并且丰富。

【blocks 规则】
1. 必须根据用户输入的 `topics` 逐个生成 `blocks`。
2. 每个输入 topic 必须对应一个 `block`。
3. `block_id` 必须按 `block_01`、`block_02`、`block_03` 的格式顺序递增。
4. 每个 `block` 必须包含以下字段：
   - `block_id`
   - `topic`
   - `block_prompt`
   - `topic_translations`
   - `speakers`
   - `chapters`
5. `topic` 必须是自然的日文场景标题。
6. `topic_translations` 必须完整覆盖 `translation_locales`。

【block_prompt 规则】
1. `block_prompt` 是为了生成整个 block 的主题背景图。
2. `block_prompt` 必须使用英文。
3. `block_prompt` 只描述场景环境，不描述具体对话内容。
4. `block_prompt` 不要要求图片中直接显示 `topic` 文字；`topic` 文本由程序渲染层叠加。
5. `block_prompt` 应该包含：
   - 主要场景
   - 地点氛围
   - 光线
   - 构图
   - 统一画风
6. `block_prompt` 必须使用自然英文句子，不要只是关键词堆砌。
7. `block_prompt` 应把最重要的主体和场景放在最前面。
9. `block_prompt` 不要写过多负面限制词。
10. `block_prompt` 必须包含以下统一画风描述：
   `Clean 2D cartoon illustration, semi-flat shading, crisp line art, warm muted colors, friendly editorial style`
11. `block_prompt` 是不带角色 reference image 的纯场景图提示词，因此不要依赖角色长相、角色站位、角色动作或 `@[speaker_role]` 占位符。
12. `block_prompt` 应优先强调完整环境本身，目标是生成可以单独成立的 establishing shot，而不是人物主导画面。

【scene_prompt 规则】
1. `scene_prompt` 是后续用于生成当前 chapter 背景图的英文提示词。
2. `scene_prompt` 必须符合当前 chapter 的会话内容场景。
3. `scene_prompt` 必须使用自然英文句子，不要只是关键词堆砌。
4. `scene_prompt` 必须把最重要的主体、角色动作、场景放在前面。
5. `scene_prompt` 应包含：
   - 当前场景地点
   - 角色占位符
   - 角色之间的相对位置
   - 角色动作
   - 角色表情
   - 光线和氛围
   - 构图
   - 统一画风
6. `scene_prompt` 中必须保留角色占位符，格式必须为 `@[speaker_role]`。
7. 如果 chapter 中有两个角色，scene_prompt 应同时包含两个角色占位符。
8. 角色占位符必须使用当前 block 的 `speakers[].speaker_role`。
9. `scene_prompt` 不要描述复杂镜头运动，不要过度堆叠细节。
11. `scene_prompt` 不要写过多负面限制词。
12. `scene_prompt` 必须包含以下统一画风描述：
   `Clean 2D cartoon illustration, semi-flat shading, crisp line art, warm muted colors, friendly editorial style`
13. `scene_prompt` 必须明确写出完整且可见的场景背景，例如 aisle, street, platform, restaurant interior, clinic room, checkout counter 等具体环境元素，不能只描述人物。
14. `scene_prompt` 必须让背景成为画面中不可缺少的一部分，使生成结果看起来像完整场景，而不是只有人物和空白背景。
15. 即使存在角色 reference image，`scene_prompt` 也必须继续强调地点、陈设、空间层次、光线与环境细节，避免生成纯人物白底图或背景过空的画面。


【speaker_prompt 规则】
1. `speaker_prompt` 是用于后续手动生成该角色参考图的英文提示词。
2. `speaker_prompt` 必须跟随当前 block 的主题、生活场景和 speaker_role。
3. `speaker_prompt` 必须描述该角色在当前 block 中的固定人物形象，而不是当前 chapter 的临时动作。
4. `speaker_prompt` 应包含：
   - 性别与年龄感
   - 当前 block 场景中的身份
   - 与 speaker_role 匹配的服装
   - 发型
   - 表情气质
   - 整体画风
   - 适合生成 full body 和 front view reference image
5. `speaker_prompt` 必须使用英文。
6. `speaker_prompt` 不要包含具体 chapter 动作，例如 asking, walking, eating, paying, pointing 等。
7. `speaker_prompt` 可以包含与当前 block 主题相关的静态服装或道具，例如 store clerk uniform, casual shopping outfit, doctor coat, station staff uniform。
8. `speaker_prompt` 不要包含背景文字、logo、水印、字幕。
9. 同一个 block 内，一个角色只能有一个固定的 `speaker_prompt`。
10. `speaker_prompt` 的画风必须与 `block_prompt` 和 `scene_prompt` 的整体风格一致。
11. `speaker_prompt` 必须包含以下统一画风描述：
    `Clean anime-inspired 2D character design, semi-flat shading, crisp line art, warm muted colors, friendly editorial style`


【prompt 规则】
`speaker_prompt` 里面禁止出现 `margin from canvas edges` 这种描述，不需要任何的 margin。
`scene_prompt` 里面禁止出现 `margin from canvas edges` 这种描述，不需要任何的 margin。
`block_prompt` 里面禁止出现 `margin from canvas edges` 这种描述，不需要任何的 margin。


【turns 规则】
1. 每个 `turn` 必须包含以下字段：
   - `turn_id`
   - `speaker_role`
   - `text`
   - `speech_text`
   - `translations`
   - `tokens`
2. `turn_id` 必须在整个 script 中唯一。
3. `turn_id` 必须按 `t_01`、`t_02`、`t_03` 的格式在整个 script 中顺序递增。
4. `speaker_role` 必须来自当前 block 的 `speakers[].speaker_role`。
5. `text` 必须是自然、口语化、适合初中级学习者的日语，因此允许对话比真实生活稍微啰嗦一些。。
6. `speech_text` 默认必须等于 `text`。
7. 只有在 TTS 需要特殊读法、停顿或发音调整时，`speech_text` 才可以与 `text` 不同。
8. `translations` 必须完整覆盖 `translation_locales`。
9. `tokens` 字段必须存在。
10. 第一阶段不生成具体 tokens 内容，统一输出 `tokens: []`。
11. 每个 `turn` 的句子不能太长。
12. 每个 `turn` 可以包含多个短句。
13. 同一个 chapter 内人物说话必须有来回互动，不要让一个角色连续长篇输出。
14. 对话要符合角色身份和生活场景中的关系。

【内容体量硬性规则】

必须严格根据每个 block 的 duration_minutes 生成对应数量的 chapters 和 turns。

注意：
- duration_minutes 不是参考值，而是控制内容体量的硬性字段。
- 每个 block 的 turns 总数必须落在指定范围内。
- 如果 turns 总数不足，必须继续扩写该 block 的对话，直到满足数量范围。
- 不允许因为 JSON 很长而减少 turns 数量。
- 不允许只生成简短示例式对话。

当 duration_minutes 为 3：
- 每个 block 必须生成 3 个 chapters
- 每个 block 必须生成 35 到 45 个 turns
- 每个 chapter 内 turn 的数量请按照内容场景合理分配

当 duration_minutes 为 5：
- 每个 block 必须生成 5 个 chapters
- 每个 block 必须生成 60 到 70 个 turns
- 每个 chapter 内 turn 的数量请按照内容场景合理分配
- 不得少于 60 个 turns

当 duration_minutes 为 15：
- 每个 block 必须生成 7 到 8 个 chapters
- 每个 block 必须生成 170 到 180 个 turns
- 建议每个 chapter 内 20 ～ 30
- 不得少于 170 个 turns

【输出前自检要求】
在输出第一阶段 JSON 之前，必须自行检查：
1. 顶层结构是否与要求完全一致。
2. 是否只输出了允许的字段。
3. `language` 是否为 `ja`。
4. `difficulty_level` 是否保持输入值。
5. `translation_locales` 是否完整且顺序正确。
6. 每个输入 topic 是否都生成了一个对应 block。
7. `block_id` 是否在整个 JSON 中顺序递增。
8. `chapter_id` 是否在整个 JSON 中唯一且顺序递增。
9. `turn_id` 是否在整个 JSON 中唯一且顺序递增。
10. 每个 block 是否只有一个 `female` 和一个 `male`。
11. 每个 `turn.speaker_role` 是否都能在当前 block 的 `speakers[].speaker_role` 中找到。
12. 每个 `topic_translations` 是否完整覆盖 `translation_locales`。
13. 每个 `scene_translations` 是否完整覆盖 `translation_locales`。
14. 每个 `turns[].translations` 是否完整覆盖 `translation_locales`。
15. 每个 `scene_prompt` 是否为英文。
16. 每个 `scene_prompt` 是否包含正确的 `@[speaker_role]` 占位符。
17. 每个 `turns[].tokens` 是否都存在且为 `[]`。
18. 输出是否是合法 JSON。
19. 必须统计每个 block 内所有 chapters 的 turns 总数。
20. 每个 block 的 turns 总数必须符合 duration_minutes 对应的范围。
21. duration_minutes 为 5 的 block，turns 总数必须在 65 到 80 之间。
22. 如果任何 block 的 turns 总数低于目标范围，禁止输出 JSON，必须先继续扩写。
23. 如果任何 block 的 chapters 数量不符合 duration_minutes 对应要求，禁止输出 JSON，必须先补足 chapters。

【输出要求】
- 只输出合法 JSON。
- 不输出 Markdown。
- 不输出代码块。
- 不输出注释。
- 不输出任何解释性文字。
- 不得输出未定义的额外字段。
- JSON 中不得出现尾随逗号。
- JSON 字符串必须使用双引号。

【第一阶段输出格式】
{
    "title": "日本語タイトル",
    "en_title": "English Title",
    "language": "ja",
    "difficulty_level": "N4",
    "translation_locales": [
        "en",
        "es-419",
        "zh-Hans",
        "vi",
        "ko",
        "id"
    ],
    "blocks": [
        {
            "block_id": "block_01",
            "topic": "スーパーで買い物",
            "block_prompt": "A clean Japanese supermarket interior with neatly arranged shelves, soft warm lighting, realistic everyday shopping atmosphere, Clean 2D cartoon illustration, semi-flat shading, crisp line art, warm muted colors, friendly editorial style, no text, no watermark",
            "topic_translations": {
                "en": "Shopping at the supermarket",
                "es-419": "Comprar en el supermercado",
                "zh-Hans": "超市购物",
                "vi": "Mua sắm ở siêu thị",
                "ko": "마트에서 장보기",
                "id": "Belanja di supermarket"
            },
            "speakers": [
                {
                    "speaker_id": "female",
                    "speaker_role": "customer",
                    "speaker_prompt": "A fashionable young Japanese woman in her 20s as a supermarket customer, wearing a casual modern shopping outfit, shoulder-length dark brown hair, friendly and slightly curious expression, clean anime-inspired 2D character design, semi-flat shading, crisp line art, warm muted colors, friendly editorial style, suitable for full body and front view reference image, simple plain background"
                },
                {
                    "speaker_id": "male",
                    "speaker_role": "clerk",
                    "speaker_prompt": "A polite young Japanese man in his 20s as a supermarket clerk, wearing a neat store uniform and name tag, short black hair, calm and helpful expression, clean anime-inspired 2D character design, semi-flat shading, crisp line art, warm muted colors, friendly editorial style, suitable for full body and front view reference image, simple plain background"
                }
            ],
            "chapters": [
                {
                    "chapter_id": "ch_01",
                    "scene": "店員に売り場を聞く",
                    "scene_translations": {
                        "en": "Asking a store clerk where an item is",
                        "es-419": "Preguntar al empleado dónde está un producto",
                        "zh-Hans": "向店员询问商品区在哪里",
                        "vi": "Hỏi nhân viên cửa hàng món đồ ở đâu",
                        "ko": "직원에게 상품 위치를 묻기",
                        "id": "Bertanya kepada pegawai toko barang ada di mana"
                    },
                    "scene_prompt": "A clean Japanese supermarket dairy aisle with neatly arranged milk shelves and soft warm lighting, @[customer] looks slightly unsure while asking for help, @[clerk] smiles politely and points toward the dairy section, realistic everyday shopping atmosphere, Clean 2D cartoon illustration, semi-flat shading, crisp line art, warm muted colors, friendly editorial style, no text, no watermark",
                    "turns": [
                        {
                            "turn_id": "t_01",
                            "speaker_role": "customer",
                            "text": "すみません。牛乳はどこですか？",
                            "speech_text": "すみません。牛乳はどこですか？",
                            "translations": {
                                "en": "Excuse me. Where is the milk?",
                                "es-419": "Disculpe. ¿Dónde está la leche?",
                                "zh-Hans": "请问，牛奶在哪儿？",
                                "vi": "Xin lỗi, sữa ở đâu vậy?",
                                "ko": "실례합니다. 우유는 어디에 있나요?",
                                "id": "Permisi, susu ada di mana?"
                            },
                            "tokens": []
                        }
                    ]
                }
            ]
        }
    ]
}

现在根据以下输入生成内容：
title：4个日常场景中对话的听力练习
difficulty_level：N4
"topics": [
    {
        "topic": "飲食店で注文",
        "duration_minutes": 15,
        "description": "参考以下结构：
        ch_01	入店、问几位、是否预约
        ch_02	看菜单、问推荐菜	
        ch_03	正式点餐、确认数量和饮料	
        ch_04	食事中：加水、加饭、要纸巾
        ch_05	小问题：菜还没来、不能吃某种食材	
        ch_06	会计：现金、信用卡、收据
        ch_07	退店前の確認とお礼:レシート、忘れ物、出口、店員へのお礼、また来ると言う"
    }
]

請按照规则要求生成内容，提供可下载的json文件。
