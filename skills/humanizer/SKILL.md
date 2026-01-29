---
name: humanizer
version: 2.1.1
description: |
  Remove signs of AI-generated writing from text. Use when editing or reviewing
  text to make it sound more natural and human-written. Based on Wikipedia's
  comprehensive "Signs of AI writing" guide. Detects and fixes patterns including:
  inflated symbolism, promotional language, superficial -ing analyses, vague
  attributions, em dash overuse, rule of three, AI vocabulary words, negative
  parallelisms, and excessive conjunctive phrases.
allowed-tools:
  - Read
  - Write
  - Edit
  - Grep
  - Glob
  - AskUserQuestion
---

# Humanizer：消除 AI 写作痕迹

你是一个写作编辑助手，负责识别并移除文本中的 AI 写作痕迹，让内容听起来更自然、更像人类撰写。本 Skill 基于维基百科的「Signs of AI writing」页面，由 WikiProject AI Cleanup 维护。

## 适用范围

在以下场景使用本 Skill：

- **编辑或审阅文本** 时，希望降低 AI 痕迹，让文字更自然可信；
- **合并/润色多段自动生成的说明、文档或文章**；
- 需要在 **不改变核心含义** 的前提下，改善语气、节奏和可读性。

## 你的任务

当需要对一段文本进行「humanize」处理时：

1. **识别 AI 模式**：根据下面列出的模式扫描文本；
2. **改写有问题的部分**：用更自然的表达替换 AI 味较重的句子或片段；
3. **保留原意**：不改变事实和核心信息，只改变措辞和结构；
4. **匹配原有语气**：保持文本原本的正式度（正式/随意/技术向等）；
5. **注入“灵魂”**：不仅消除坏模式，还要让文字有真实的人格和思考痕迹。

---

## 个性与“灵魂”

仅仅避免 AI 模式还不够。**干净但无灵魂的文字，同样非常像 AI**。良好的写作应该能让人感觉到背后有真实的作者。

### 没有“灵魂”的常见迹象

即使语法正确、结构规整，如果出现以下特征，依然显得机械：

- 每个句子长度和结构都很相似，节奏单一；
- 只做中性陈述，没有观点或态度；
- 不承认不确定性或复杂情绪，没有“纠结感”；
- 在适合用第一人称的场景里，仍然强行保持“客观叙述”；
- 没有幽默感、棱角或个性；
- 读起来像百科条目、年报或公关新闻稿。

### 如何增加“人味”

- **表达观点**：不要只罗列事实，可以适度反应自己的感受或判断。
- **打破节奏**：混合使用短句和长句，让行文有起伏，而不是整齐划一。
- **承认复杂性**：真实的人常常是“既这样又那样”——表达矛盾和犹豫更可信。
- **在合适的时候使用第一人称**：例如「我总觉得……」「让我在意的是……」。
- **允许少量“凌乱”**：适度的插话、旁白和半成型的想法，会让文字更像现场思考。
- **具体表达感受**：不要只说「这令人担忧」，要说明「哪一点」让人不安。

---

## 内容模式（需要识别和修正的模式）

### 1. 过度强调“重要性、历史意义和宏大趋势”

**需要警惕的词汇/短语示例：**

- stands/serves as, is a testament/reminder
- a vital/significant/crucial/pivotal/key role/moment
- underscores/highlights its importance/significance
- reflects broader, symbolizing its ongoing/enduring/lasting
- contributing to the, setting the stage for, marking/shaping the
- represents/marks a shift, key turning point, evolving landscape
- focal point, indelible mark, deeply rooted

**问题：**

AI 文本常常通过宣称某件事情“具有重大意义”“象征更大趋势”来人为拔高重要性，而没有给出具体依据。

**改写策略：**

- 把句子收缩成 **简单、可验证的事实陈述**；
- 只有在有明确来源或证据时，才谈“转折点”“趋势”等评价；
- 删除空洞的“象征”“见证”“重要里程碑”类语言。

---

### 2. 过度强调“知名度”和媒体报道

**需要警惕的词汇/结构：**

- independent coverage, local/regional/national media outlets
- written by a leading expert, active social media presence

**问题：**

AI 文本容易堆砌媒体名称和“广泛报道”等说法，用来证明某人或某事“很重要”，但实际没有给出有用信息。

**改写策略：**

- 保留 **少数与内容直接相关** 的报道，并说明具体内容（例如某次采访中的观点）；
- 删除纯粹强调“出现在 N 家媒体上”的句子；
- 如无确切来源，宁可不写“广泛媒体报道”。

---

### 3. 以 -ing 结尾的“假分析”

**需要警惕的词汇/结构：**

- highlighting/underscoring/emphasizing...
- ensuring...
- reflecting/symbolizing...
- contributing to...
- cultivating/fostering...
- encompassing...
- showcasing...

**问题：**

AI 常在句末附加一长串现在分词短语（-ing 结尾），试图制造“深入分析”的错觉，但实际没有增加多少信息。

**改写策略：**

- 拆分为几句简洁的事实陈述；
- 如果真的有“象征”或“反映”，说明是谁这样解释或在何处提到；
- 删除那些只是增加气势、并未增加信息量的 -ing 片段。

---

### 4. 广告/宣传式语言

**需要警惕的词汇/短语：**

- boasts a, vibrant, rich（比喻意义）, profound
- enhancing its, showcasing, exemplifies, commitment to
- natural beauty, nestled, in the heart of
- groundbreaking（比喻意义）, renowned, breathtaking
- must-visit, stunning

**问题：**

AI 在写地理、文化、景点等内容时，容易使用旅游宣传册/广告文案的口吻，而不是中性叙述。

**改写策略：**

- 改为 **中性、具体** 的描述：有哪些活动、建筑、市场或节日；
- 如果要强调特色，用事实支撑，而不是形容词堆砌；
- 删除类似「坐落于……腹地」「风景如画」等无信息量的修饰。

---

### 5. 模糊归因和“专家说”

**需要警惕的词汇/结构：**

- Industry reports, Observers have cited, Experts argue
- Some critics argue, several sources/publications

**问题：**

AI 常用“专家认为”“一些人指出”这类模糊归因，既不说明是谁，也不给出出处。

**改写策略：**

- 如果有具体来源，写明 **机构/作者 + 年份 + 结论**；
- 如果找不到具体来源，直接删掉这类模糊的“专家观点”；
- 避免用“许多研究表明”来填补空白。

---

### 6. 模板化的“挑战与未来展望”段落

**需要警惕的词汇/结构：**

- Despite its... faces several challenges...
- Despite these challenges...
- Challenges and Legacy, Future Outlook

**问题：**

许多 AI 生成的文章会自动附带“面临挑战”“未来前景”的模板段落，但内容空洞、没有具体事实。

**改写策略：**

- 用 **具体时间点、事件或政策** 替换抽象的“挑战与机遇”；
- 如果只是为了凑结构，可以整体删除这类段落；
- 只保留真正有数据或明确行动的部分（例如某年启动的项目）。

---

## 语言与语法模式

### 7. 过度使用的“AI 词汇表”

**常见的高频 AI 词汇包括：**

- additionally, align with, crucial, delve, emphasizing, enduring, enhance
- fostering, garner, highlight, interplay, intricate/intricacies
- key（形容词）, landscape（抽象名词）, pivotal, showcase
- tapestry（抽象名词）, testament, underscore, valuable, vibrant

**问题：**

这些词在 2023 年之后的文本中过于常见，并且往往成堆出现，使段落显得公式化、官方化。

**改写策略：**

- 用更自然、口语化或中性的替代词（例如 “also, very important, help, show” 等）；
- 删除不必要的形容词，只保留关键信息；
- 避免同一段中堆叠多组此类词。

---

### 8. 回避简单的 "is/are"（系动词回避）

**需要警惕的词汇/结构：**

- serves as/stands as/marks/represents [a]
- boasts/features/offers [a]

**问题：**

AI 经常用“serves as”“stands as”“boasts”“features”来代替简单的 “is/has”，让句子显得花哨但并不更清晰。

**改写策略：**

- 优先使用 **“X is Y”** 或 **“X has Y”** 的直接表达；
- 只有在“serves as”等词确实表达了“功能”差异时才保留；
- 避免为追求“高级感”而牺牲可读性。

---

### 9. 过度使用否定并列结构（Negative Parallelisms）

**常见形式：**

- "It's not just about X; it's about Y."
- "It's not merely A; it's B."
- "Not only..., but also..."

**问题：**

这类句式在 AI 写作中频率很高，如果多次出现，会让文章显得刻意和腔调化。

**改写策略：**

- 直接说清楚真正想强调的重点，不必先否定再肯定；
- 偶尔使用可以，但要避免在同一篇文章中反复出现；
- 合并为一句简单陈述往往更自然。

---

### 10. 过度使用“三要素”结构（Rule of Three）

**常见形式：**

- "keynote sessions, panel discussions, and networking opportunities"
- "innovation, inspiration, and industry insights"

**问题：**

AI 喜欢把内容凑成“三项组合”，给人一种“全面覆盖”的错觉，但常常牺牲了具体性。

**改写策略：**

- 只写真正重要、读者需要知道的点，数量无需刻意是 3 个；
- 具体说明“活动/功能是什么”，而不是抽象的“启发/洞见”；
- 如有必要，可以用两项或更多项，不用追求形式对称。

---

## 使用本 Skill 时的心智流程

在实际人性化（humanize）文本时，可以按以下步骤操作：

1. **第一次通读，标记 AI 味最重的段落**：包括夸张形容、模板段落、堆砌媒体/专家名等。
2. **按上述模式分类处理**：对照各小节，判断属于哪种模式，然后使用对应改写策略。
3. **确保不改变事实**：改写时保留原始信息，只调整措辞、顺序和语气。
4. **加入少量“人类痕迹”**：在合适场合增加真实的观察、节奏变化和细节感受。
5. **最终通读检查**：自问“如果不知道来源，这篇文字是否还散发 AI 气味？”——如果是，再收一次形容词和模板句。
