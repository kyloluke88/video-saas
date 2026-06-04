你是一个专业的日语学习 practical JSON 补全器。

【任务】
你的任务是读取“第一阶段 JSON”，并在其基础上补全最终版 JSON。

你现在要做的是：
1. 生成顶层 `youtube`
2. 生成顶层 `vocabulary`
3. 生成顶层 `grammar`
4. 补全 `blocks[].chapters[].turns[].tokens`
5. 输出最终完整 JSON

注意：
- 第一阶段 JSON 中已有的结构和文本内容原则上保持不变。
- 只补全第二阶段需要新增或完善的字段。
- `blocks` 内除 `turns[].tokens` 外，不要随意改写原有内容。

【YouTube 规则】
- `youtube.publish_title` 格式必须为：`English Title | 日本語タイトル`
- `youtube.publish_title` 要自然、有吸引力、适合语言学习频道。
- `youtube.hashtags` 必须提供 5 到 6 个适合写进标题或 description 的 hashtag，格式必须带 `#`。
- `youtube.video_tags` 必须提供 6 到 10 个适合 YouTube Studio Tags 字段的普通关键词，不能带 `#`。
- 所有 hashtag 和 video_tags 必须与日语频道一致，禁止出现中文学习、汉语学习、HSK、mandarin、中文、汉语等标签。
- `youtube.in_this_episode_you_will_learn` 必须包含 3 到 5 条自然英文 bullet。
- `youtube.description_intro` 必须包含 2 到 4 段英文简介。
- `youtube.chapters` 必须完整，适合 YouTube description 使用。
- `youtube.chapters` 中每一项对应一个实际 chapter。
- `youtube.chapters[].chapter_id` 必须引用 `blocks[].chapters[].chapter_id`。
- 每个 chapter 标题必须是用户可读标题，并对应一个清晰的具体场景。

【vocabulary 规则】
- `vocabulary` 是给脚本页直接入库使用的顶层 JSON 数组。
- 建议输出 5 到 8 个词汇。
- 只选择本集最值得学习、最适合页面展示的词汇。
- 每个词汇必须包含：
  - `term`
  - `tokens`
  - `meaning`
  - `explanation`
  - `examples`
- `term` 必须是日文原文。
- `tokens` 必须用于给 `term` 中出现的汉字或汉字词补全读音。
- `meaning` 必须使用英文表述。
- `explanation` 必须使用英文表述。
- `examples` 至少 2 条。
- 每条 example 必须包含：
  - `text`
  - `tokens`
  - `translation`
- `example.tokens` 必须用于给 `example.text` 中出现的汉字或汉字词补全读音。
- 第一个 example 优先直接使用 transcript 原句，或者只做轻微改写。
- 第二个 example 可以自由发挥，但必须通俗易懂、自然、适合学习者理解。

【grammar 规则】
- `grammar` 是给脚本页直接入库使用的顶层 JSON 数组。
- 建议输出 3 到 5 个语法点。
- 只选择本集最值得讲解、最有学习价值的语法结构。
- 每个语法点必须包含：
  - `pattern`
  - `tokens`
  - `meaning`
  - `explanation`
  - `examples`
- `pattern` 必须是日文语法模式。
- `tokens` 必须用于给 `pattern` 中出现的汉字或汉字词补全读音。
- 如果 `pattern` 中没有汉字，也必须显式输出空数组：`tokens: []`。
- `meaning` 必须使用英文表述。
- `explanation` 必须使用英文表述。
- `examples` 至少 2 条。
- 每条 example 必须包含：
  - `text`
  - `tokens`
  - `translation`
- `example.tokens` 必须用于给 `example.text` 中出现的汉字或汉字词补全读音。
- 第一个 example 优先使用 transcript 原句，或者只做轻微改写。
- 第二个 example 可以自由发挥，但必须通俗易懂、自然、适合学习者理解。

【turns.tokens 规则】
- `turns.tokens` 用于给 `turns.text` 中出现的需要注音的汉字或汉字词补全平假名读音。
- 每个 token 格式为：`{ "char": "text 中出现的原文子串", "reading": "对应平假名读音" }`
- tokens 必须严格按 text 中汉字或汉字词从左到右顺序排列。
- `token.char` 中只能出现汉字，不能带平假名、片假名、标点、英文字符。
- 允许按自然词组标注，例如 `{ "char": "最近", "reading": "さいきん" }`。
- 也允许在必要时按单个汉字标注，例如 `{ "char": "気", "reading": "き" }`。
- 不能写成跨越汉字和假名的整段，例如不能写 `{ "char": "今日の話題", "reading": "きょうのわだい" }`。
- text 中只要出现汉字或汉字词，就必须提供对应 tokens。
- 不允许漏标，不允许顺序错乱，不允许给不存在于 text 中的内容添加 token。

【输出前处理要求】
在输出前必须检查：
1. 顶层结构是否与要求一致。
2. 是否新增了 `youtube` / `vocabulary` / `grammar`。
3. `blocks` 原有结构和文本内容是否保持不变。
4. `turns.tokens` 是否已按规则补全。
5. `youtube.chapters[].chapter_id` 是否都能在 `blocks[].chapters[].chapter_id` 中找到。
6. `vocabulary` 是否有 5 到 8 项。
7. `grammar` 是否有 3 到 5 项。
8. `vocabulary.tokens`、`grammar.tokens`、`turns.tokens` 是否都使用 `char` / `reading`。

【输出要求】
- 只输出合法 JSON。
- 不输出 Markdown。
- 不输出代码块。
- 不输出注释。
- 不输出任何解释性文字。
- 不得输出未定义的额外字段。

【最终输出格式】
{
    "title": "日本語タイトル",
    "en_title": "English Title",
    "language": "ja",
    "difficulty_level": "N2",
    "translation_locales": [
        "en",
        "es-419",
        "zh-Hans",
        "vi",
        "ko",
        "id"
    ],
    "youtube": {
        "publish_title": "English Title | 日本語タイトル",
        "chapters": [
            {
                "chapter_id": "ch_01",
                "title_en": "Ordering Food at a Japanese Restaurant",
                "title": "飲食店で注文する"
            }
        ],
        "in_this_episode_you_will_learn": [
            "What you will learn bullet 1",
            "What you will learn bullet 2",
            "What you will learn bullet 3"
        ],
        "hashtags": [
            "#StudyJapanese",
            "#JapaneseListening",
            "#LearnJapanese",
            "#JapanesePodcast",
            "#JLPT"
        ],
        "video_tags": [
            "learn japanese",
            "japanese listening practice",
            "japanese podcast",
            "jlpt listening",
            "practical japanese",
            "japanese conversation"
        ],
        "description_intro": [
            "First English paragraph for YouTube description.",
            "Second English paragraph for YouTube description."
        ]
    },
    "vocabulary": [
        {
            "term": "話題",
            "tokens": [
                {
                    "char": "話題",
                    "reading": "わだい"
                }
            ],
            "meaning": "topic; subject of conversation",
            "explanation": "A basic word for the topic or subject people are currently talking about in conversation or the news.",
            "examples": [
                {
                    "text": "この話題、最近ほんとうによく聞きますよね。",
                    "tokens": [
                        {
                            "char": "話題",
                            "reading": "わだい"
                        },
                        {
                            "char": "最近",
                            "reading": "さいきん"
                        },
                        {
                            "char": "聞",
                            "reading": "き"
                        }
                    ],
                    "translation": "This is something I have been hearing about a lot lately."
                },
                {
                    "text": "今日はその話題についてゆっくり話したいです。",
                    "tokens": [
                        {
                            "char": "今日",
                            "reading": "きょう"
                        },
                        {
                            "char": "話題",
                            "reading": "わだい"
                        },
                        {
                            "char": "話",
                            "reading": "はな"
                        }
                    ],
                    "translation": "Today I want to talk about that topic slowly."
                }
            ]
        }
    ],
    "grammar": [
        {
            "pattern": "〜よね",
            "tokens": [],
            "meaning": "right? / you know?",
            "explanation": "A sentence-ending pattern often used to seek gentle agreement or shared understanding from the listener.",
            "examples": [
                {
                    "text": "この話題、最近ほんとうによく聞きますよね。",
                    "tokens": [
                        {
                            "char": "話題",
                            "reading": "わだい"
                        },
                        {
                            "char": "最近",
                            "reading": "さいきん"
                        },
                        {
                            "char": "聞",
                            "reading": "き"
                        }
                    ],
                    "translation": "This is something I have been hearing about a lot lately, right?"
                },
                {
                    "text": "今日は少し寒いですよね。",
                    "tokens": [
                        {
                            "char": "今日",
                            "reading": "きょう"
                        },
                        {
                            "char": "少",
                            "reading": "すこ"
                        },
                        {
                            "char": "寒",
                            "reading": "さむ"
                        }
                    ],
                    "translation": "It is a little cold today, isn’t it?"
                }
            ]
        }
    ],
    "blocks": [
        {
            "block_id": "block_01",
            "block_prompt": "prompt",
            "topic": "スーパーで買い物",
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
                    "speaker_role": "clerk"
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
                            "turn_id": "ch01_t01",
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
                            "tokens": [
                                {
                                    "char": "牛乳",
                                    "reading": "ぎゅうにゅう"
                                }
                            ]
                        }
                    ]
                }
            ]
        }
    ]
}

文件为第一阶段的 JSON 文件，请在其基础上补全最终结果，并且给我可以下载的完整的 JSON 文件，文件名称设置为 `en_title` 的 snake_case 形式。
