// Traditional Chinese translations.
//
// Mirrors the zh-CN key set. For MVP this is a direct conversion;
// later we can refine regional wording (e.g. 程式 vs 程序) and add
// Taiwan/HK specific phrasing where it differs.
import type { Translation } from './zh-CN'

const zhTW: Translation = {
  app: {
    name: '知命',
    tagline: 'AI 輔助的東方命理與西方占卜',
  },
  nav: {
    home: '首頁',
    bazi: '八字命理',
    dream: '周公解夢',
    zodiac: '生肖運勢',
    huangli: '萬年曆黃曆',
    constellation: '星座',
    tarot: '塔羅',
    compatibility: '配對合盤',
    account: '我的',
    admin: '後台',
  },
  lang: {
    'zh-CN': '简体中文',
    'zh-TW': '繁體中文',
  },
  common: {
    loading: '載入中…',
    submit: '送出',
    cancel: '取消',
    save: '儲存',
    back: '返回',
    comingSoon: '即將推出',
    free: '免費',
    paid: '付費',
    login: '登入',
    logout: '登出',
    aiInterpret: 'AI 解讀',
  },
  home: {
    heroTitle: '探索你的命運',
    heroSubtitle: '八字、解夢、生肖、黃曆、星座、塔羅——AI 為你解讀人生玄機。',
    cta: '開始測算',
    disclaimerTitle: '免責聲明',
    disclaimerBody: '本站內容僅供娛樂與文化參考，不構成任何專業建議。',
  },
  bazi: {
    title: '八字命理',
    subtitle: '輸入出生時間，排出你的四柱八字、大運與流年。',
  },
  health: {
    online: '服務在線',
    offline: '服務離線',
    version: '版本',
  },
}

export default zhTW
