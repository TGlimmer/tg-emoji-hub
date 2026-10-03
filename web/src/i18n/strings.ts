// i18n 字符串字典 —— 只覆盖 SearchPage 及其直接子组件的可见文案
// 弹窗内部正文量大，本轮只翻触发按钮+标题，内部留 TODO i18n

export type Locale = "zh" | "en";

export const strings = {
  zh: {
    // header 按钮
    header_community: "交流群",
    header_mcp: "MCP",
    header_add_pack: "添加表情包",

    // 语言切换
    lang_toggle: "EN",

    // hero
    hero_subtitle: "Telegram Premium emoji 搜索 · Bot 开发者的 custom_emoji_id 仓库",

    // SearchBar
    search_placeholder: "搜索 emoji（fire / 🔥 / heart …）",
    search_clear: "清空",

    // StatsHero 标签
    stat_emoji: "emoji",
    stat_packs: "packs",
    stat_animated: "animated",
    stat_today: "today",

    // 按分类浏览 section
    browse_by_category: "按分类浏览",
    category_prefix: "分类：",

    // 分类名
    category_emotion: "表情",
    category_animal: "动物",
    category_person: "人物",
    category_food: "食物",
    category_crypto: "加密货币",
    category_object: "物品",
    category_symbol: "符号",
    category_gesture: "手势",
    category_meme: "梗图",
    category_vehicle: "载具",
    category_nature: "自然",

    // HotPacks section
    hot_packs: "Hot Packs",

    // UsagePreview
    usage_hint: '搜索 "fire" → 复制 HTML 输出',
    usage_copy: "copy",
    usage_copied: "copied",

    // 搜索态筛选行
    filter_animated_only: "animated only",

    // EmojiGrid
    grid_found: "找到",
    grid_found_unit: "个结果",
    grid_keyboard_hint: "←↑↓→ 导航 · Enter 复制 HTML · 点击查看 4 种格式",
    grid_idle_title: "开始输入关键词或选择分类浏览",
    grid_idle_sub: "支持 fire / 🔥 / heart 等查询，或点击上方分类 chip 浏览。按 / 快速聚焦搜索框。",
    grid_error_title: "搜索失败",
    grid_error_sub: "请稍后重试",
    grid_retry: "重试",
    grid_empty_sub: "试试其他关键词或切换表情包/分类筛选。也可以在右上角添加新的表情包。",
    grid_empty_query: (q: string) => `没有找到 "${q}" 的结果`,
    grid_empty_category: (cat: string) => `分类 "${cat}" 暂无结果`,
    grid_empty_pack: (pack: string) => `表情包 "${pack}" 暂无结果`,
    grid_empty_fallback: "没有找到结果",

    // 搜索态分类 chip（可移除）
    active_category_label: "分类",
    active_category_remove: "移除分类筛选",

    // footer
    footer_search_hint: "搜索",
    footer_copy_hint: "复制 HTML",

    // McpDialog 触发按钮 & 标题
    mcp_btn: "MCP",
    mcp_dialog_title: "MCP 接入",

    // AddPackDialog 触发按钮 & 标题
    add_pack_btn: "添加表情包",
    add_pack_dialog_title: "添加自定义表情包",
  },
  en: {
    // header 按钮
    header_community: "Community",
    header_mcp: "MCP",
    header_add_pack: "Add Pack",

    // 语言切换
    lang_toggle: "中",

    // hero
    hero_subtitle: "Telegram Premium emoji search · custom_emoji_id repo for Bot devs",

    // SearchBar
    search_placeholder: "Search emoji（fire / 🔥 / heart …）",
    search_clear: "Clear",

    // StatsHero 标签
    stat_emoji: "emoji",
    stat_packs: "packs",
    stat_animated: "animated",
    stat_today: "today",

    // 按分类浏览 section
    browse_by_category: "Browse by Category",
    category_prefix: "Category: ",

    // 分类名
    category_emotion: "emotion",
    category_animal: "animal",
    category_person: "person",
    category_food: "food",
    category_crypto: "crypto",
    category_object: "object",
    category_symbol: "symbol",
    category_gesture: "gesture",
    category_meme: "meme",
    category_vehicle: "vehicle",
    category_nature: "nature",

    // HotPacks section
    hot_packs: "Hot Packs",

    // UsagePreview
    usage_hint: 'Search "fire" → Copy HTML output',
    usage_copy: "copy",
    usage_copied: "copied",

    // 搜索态筛选行
    filter_animated_only: "animated only",

    // EmojiGrid
    grid_found: "Found",
    grid_found_unit: "results",
    grid_keyboard_hint: "←↑↓→ navigate · Enter copy HTML · click for 4 formats",
    grid_idle_title: "Type a keyword or pick a category",
    grid_idle_sub: "Try fire / 🔥 / heart, or click a category chip above. Press / to focus the search box.",
    grid_error_title: "Search failed",
    grid_error_sub: "Please try again",
    grid_retry: "Retry",
    grid_empty_sub: "Try another keyword or change pack / category filter. You can also add a new pack via the button in the top-right.",
    grid_empty_query: (q: string) => `No results for "${q}"`,
    grid_empty_category: (cat: string) => `No results in category "${cat}"`,
    grid_empty_pack: (pack: string) => `No results in pack "${pack}"`,
    grid_empty_fallback: "No results found",

    // 搜索态分类 chip（可移除）
    active_category_label: "Category",
    active_category_remove: "Remove category filter",

    // footer
    footer_search_hint: "search",
    footer_copy_hint: "copy HTML",

    // McpDialog 触发按钮 & 标题
    mcp_btn: "MCP",
    mcp_dialog_title: "MCP Integration",

    // AddPackDialog 触发按钮 & 标题
    add_pack_btn: "Add Pack",
    add_pack_dialog_title: "Add Custom Pack",
  },
} as const satisfies Record<Locale, Record<string, string | ((...args: string[]) => string)>>;

export type StringKey = keyof (typeof strings)["zh"];
