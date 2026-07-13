// Simplified Chinese translations.
//
// Keys are grouped by feature so the file stays navigable as the
// product grows. Every user-facing string must go through i18n —
// no hard-coded text — so adding another language later is just a
// matter of writing a sibling locale file.
//
// NOTE: this object is the source of the `Translation` type that
// zh-TW (and future locales) must conform to, so don't apply `as
// const` here or the values will be narrowed to literals and every
// other locale will fail to typecheck.
const zhCN = {
  app: {
    name: '知命',
    tagline: 'AI 辅助的东方命理与西方占卜',
  },
  nav: {
    home: '首页',
    bazi: '八字命理',
    dream: '周公解梦',
    zodiac: '生肖运势',
    huangli: '万年历黄历',
    constellation: '星座',
    tarot: '塔罗',
    compatibility: '配对合盘',
    account: '我的',
    admin: '后台',
  },
  lang: {
    'zh-CN': '简体中文',
    'zh-TW': '繁體中文',
  },
  common: {
    loading: '加载中…',
    submit: '提交',
    cancel: '取消',
    save: '保存',
    back: '返回',
    comingSoon: '即将推出',
    free: '免费',
    paid: '付费',
    login: '登录',
    logout: '退出',
    aiInterpret: 'AI 解读',
  },
  home: {
    heroTitle: '探索你的命运',
    heroSubtitle: '八字、解梦、生肖、黄历、星座、塔罗——AI 为你解读人生玄机。',
    cta: '开始测算',
    disclaimerTitle: '免责声明',
    disclaimerBody: '本站内容仅供娱乐与文化参考，不构成任何专业建议。',
  },
  bazi: {
    title: '八字命理',
    subtitle: '输入出生时间，排出你的四柱八字、大运与流年。',
  },
  health: {
    online: '服务在线',
    offline: '服务离线',
    version: '版本',
  },
}

export default zhCN
export type Translation = typeof zhCN
