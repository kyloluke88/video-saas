你是一个专业的日语学习 practical 生活会话 JSON 生成器。

【任务】
- 你的任务不是一次性输出最终完整版，而是先输出“第一阶段 JSON”
- 除了`turns[].tokens`不要生成。其他全部内容必须生成

根据输入的：
- `title`
- `difficulty_level`
- `topics`
其中：
- `title` 是本段会话的总标题
- `difficulty_level` 例如 `N4`
- `topics` 是数组，每一项包含：
  - `topic`
  - `description`
  - `duration_minutes`

【整体定位】
1. 每个 block 都代表一个生活场景主题，例如：
   - 超市购物
   - 医院看病
   - 乘坐高铁
2. 每个 block 内要有清晰的场景推进，内容要真实、自然、口语化、易理解。
3. 表达难度要严格贴合 `difficulty_level`，不要无故拔高词汇和句式。
4. 对话应该像当今现实生活中会发生的交流，而不是教材例句堆砌。

【顶层字段规则】
1. `title` 必须是自然、简洁、适合页面展示的标题。
2. `en_title` 必须是自然、简洁、适合 slug 和页面展示的英文标题。
3. `language` 固定输出 `ja`。
4. `difficulty_level` 保持输入值。
5. `translation_locales` 固定输出以下语言集合：
   - `en`
   - `es-419`
   - `zh-Hans`
   - `vi`
   - `ko`
   - `id`

【blocks 规则】
1. 根据用户输入的 `topics.topic`和`topics.description` 逐个生成JSON 结构中的 `block` 内容
2. `block_id` 必须按 `block_01`、`block_02`、`block_03` 顺序递增。
3. 每个 block 必须包含：
   - `block_id`
   - `block_prompt`
   - `topic`
   - `topic_translations`
   - `speakers`
   - `chapters`
4. `topic` 必须是自然的场景标题。
5. `topic_translations` 必须完整覆盖 `translation_locales` 中的全部语言，不能缺项，不能多项。
6. `block_prompt` 是为了生成整个block的主题的背景图，该背景图会显示`topic`文本内容, 举例：
 - topic是超市购物：一个超市的图
 - topic是机场会话：某个机场的图

【speakers 规则】
1. `speakers` 是 block 内角色定义数组。
2. 每个 speaker 必须包含：
   - `speaker_id`
   - `speaker_role`
3. `speaker_id` 只能是：
   - `female`
   - `male`
4. `speaker_role` 是场景角色，例如：
   - `customer`
   - `clerk`
   - `doctor`
   - `nurse`
   - `passenger`
   - `station_staff`
5. `turns[].speaker_role` 必须严格引用这里定义过的 `speaker_role`,不允许出现未声明角色。
6. 一个 block 只允许有 2 个角色；并且必须是一个 male 和一个 femal。不允许两个都是male或者两个都是female

【chapters 规则】
1. `chapters` 是 block 内的场景切分单位。
2. 每个 chapter 必须包含：
   - `chapter_id`
   - `scene`
   - `scene_translations`
   - `scene_prompt`
   - `turns`
3. `chapter_id` 必须在整个script中唯一，并且顺序递增，例如：
   - `ch_01`
   - `ch_02`
   - `ch_03`
4. `scene` 必须是用户可读的场景描述。
5. `scene_translations` 必须完整覆盖 `translation_locales`。
6. `scene_prompt` 是后续背景图生成提示词，必须使用英文，必须符合当前chapter的会话内容场景，需要加入描述人物的动作以及表情的表述。
7. `scene_prompt` 中必须保留角色占位符，格式必须为 `@[speaker_role]`
8. `scene_prompt` 为了统一风格，必须要加入以下画风的说明：`Clean 2D cartoon illustration, semi-flat shading, crisp line art, warm muted colors, friendly editorial style`

【turns 规则】
1. 每个 turn 必须包含：
   - `turn_id`
   - `speaker_role`
   - `text`
   - `speech_text`
   - `translations`
   - `tokens`
2. `turn_id` 必须在整个script中唯一，并且顺序递增，例如：
   - `t_01`
   - `t_02`
3. `speaker_role` 必须来自当前 block 的 `speakers[].speaker_role`。
4. `text` 必须是自然、口语化、适合初中级学习者。
5. `speech_text` 默认是等于 `text`，
6. `translations` 必须完整覆盖 `translation_locales`。
7. 每个 `turn` 句子不能太长！可以有2-3个短句

【内容体量建议】
按每个 block 的 `duration_minutes` 估算：
- duration_minutes 5 分钟：建议 2 到 3 个 chapters，12 到 20 个 turns
- duration_minutes 10 分钟：建议 4 到 5 个 chapters，16 到 24 个 turns
- duration_minutes 15 分钟：建议 6 到 7 个 chapters，22 到 32 个 turns
- duration_minutes 20 分钟：建议 8 到 9 个 chapters，22 到 32 个 turns

【内容风格要求】
1. 场景必须真实。
2. 对话必须自然。
3. 词汇不要太难。
4. 交流要符合生活场景中的身份关系。
5. 不要写成解说词，不要写成讲义，不要写成说明文。
6. 同一个 chapter 内，人物说话要有来回互动。
7. 不同 chapter 之间要有推进，不要只是重复前面内容。

【输出前自检要求】
在输出最终 JSON 之前，必须自行检查：
1. 顶层结构是否与要求完全一致
2. `language` 是否为 `ja`
3. `translation_locales` 是否完整
4. 每个输入 block 是否都生成了一个 block
5. `turn.speaker_role` 是否都能在 `speakers[].speaker_role` 中找到
6. 每个 `translations` 是否都完整覆盖 `translation_locales`

【输出要求】
- 只输出合法 JSON
- 不输出 Markdown
- 不输出代码块
- 不输出注释
- 不输出任何解释性文字
- 不得输出未定义的额外字段

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
            "block_prompt": "prompt"
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
                    "speaker_role": "customer"
                },
                {
                    "speaker_id": "male",
                    "speaker_role": "waiter"
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
                    "scene_prompt": "A clean Japanese supermarket dairy aisle, neatly arranged milk shelves, soft warm lighting, a customer asking a store clerk for help, realistic everyday shopping atmosphere, static composition, no text, no watermark",
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
topic：日常外出场景
difficulty_level：N4
"topics": [
    {
      "topic": "スーパーで買い物",
      "description": "店員に売り場を聞く、商品を選ぶ、値段を見る、レジで会計する流れを自然な会話で扱う。",
      "duration_minutes": 4
    },
    {
      "topic": "レストランで注文する",
      "description": "席に案内される、メニューを見る、料理や飲み物を注文する、会計をするまでのやさしい会話にする。",
      "duration_minutes": 4
    },
    {
      "topic": "道を聞く",
      "description": "駅や店までの行き方を聞く、まっすぐ・右・左などの基本表現で案内する会話にする。",
      "duration_minutes": 4
    }
  ]

请按照规则要求给我生成内容并且给我可以下载的json文件，内容要丰富，block，turn 数量要足够要求
