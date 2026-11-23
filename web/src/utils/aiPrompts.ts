import type { AIPromptSettings } from '@/api/types';

/**
 * REQ-042：系统默认 AI 任务指令。真实后端通过 settings.ai.prompt_defaults
 * 返回同一份文案；这里只用于 mock、首屏默认值和类型完备。
 */
export const DEFAULT_AI_PROMPTS: AIPromptSettings = {
  summary_bullets: '输出 3–6 条中文要点，每行一条，不要编号或 Markdown 标记。',
  summary_one: '用一句中文概括，不要 Markdown。',
  digest_batch: '请为每一条写一句中文要点，严格控制在 40 个汉字以内；合并同一事件的不同说法；只输出「- 编号. 要点」列表，不要标题、来源、链接或额外评论。',
  digest_merge: [
    '1. 开头用 2-3 句总述本期重点，不超过 150 字；',
    '2. 按 3-6 个主题分节，每节 1-3 条短要点，每条不超过 50 字；',
    '3. 合并重复或相似内容，不要把每篇文章逐条列出；',
    '4. 不输出文章清单、链接、来源和时间；',
    '5. 只使用给定小结，不编造；全文控制在 900-1400 字。',
  ].join('\n'),
};

export const EMPTY_AI_PROMPTS: AIPromptSettings = {
  summary_bullets: '',
  summary_one: '',
  digest_batch: '',
  digest_merge: '',
};
