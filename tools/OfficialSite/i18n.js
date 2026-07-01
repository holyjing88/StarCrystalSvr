(function () {
  'use strict';

  var STORAGE_KEY = 'official-site-lang';
  var DEFAULT_LANG = 'zh';

  var I18N = {
    zh: {
      meta_title: 'StarLaneInfinite Technology · 星澜无界科技',
      meta_desc: 'StarLaneInfinite Technology — SCPlayer 多媒体播放器与星澜小游戏平台，吸血鬼宿舍、最强大脑、拯救主公等精品休闲游戏。',
      logo_sub: '星澜无界科技',
      nav_products: '产品',
      nav_games: '游戏',
      nav_about: '关于',
      nav_contact: '联系',
      lang_label: '语言',
      hero_eyebrow: 'Multimedia · Casual Games · Unity & H5',
      hero_title: '连接内容与玩法的<br />无界体验',
      hero_lead: '<strong>StarLaneInfinite Technology</strong>（星澜无界科技）专注多媒体播放与轻量游戏生态：自研 <strong>SCPlayer</strong> 播放器与 <strong>星澜小游戏平台</strong>，持续推出吸血鬼宿舍、最强大脑、拯救主公等原创小游戏。',
      hero_btn_products: '了解产品',
      hero_btn_games: '浏览游戏',
      products_title: '核心产品',
      products_desc: '从播放到游玩，一套技术栈贯通内容与运营。',
      sc_tag: '多媒体',
      sc_sub: '多媒体播放器 · Unity',
      sc_desc: '基于 Unity 引擎打造的高性能多媒体播放器，支持常见音视频格式与流畅交互界面，适用于教育、展示、嵌入式播放等场景，可与星澜内容生态无缝衔接。',
      sc_p1: 'Unity 跨平台渲染与 UI 体系',
      sc_p2: '专注播放体验与扩展插件架构',
      sc_p3: '面向企业与内容方定制部署',
      plat_tag: '平台',
      plat_name: '星澜小游戏平台',
      plat_sub: '统一发行 · H5 热更新',
      plat_desc: '统一账号、金币福利与游戏目录的轻量发行平台。客户端动态拉取游戏列表，H5 包体在线更新，运营后台可配置上架、版本与渠道，降低发版成本。',
      plat_p1: '多游戏一站式登录与任务体系',
      plat_p2: 'H5 / Unity 混合接入',
      plat_p3: 'IDIP 运营台与数据审计',
      games_title: '精品小游戏',
      games_desc: '星澜平台持续上线的原创与联运作品，即点即玩、碎片时间友好。',
      g1_tag: '休闲 · 养成',
      g1_name: '吸血鬼宿舍',
      g1_desc: '趣味宿舍题材休闲小游戏，轻松上手，适合全年龄段碎片娱乐。',
      g2_tag: '益智 · 脑力',
      g2_name: '最强大脑',
      g2_desc: '挑战观察力与逻辑思维的益智关卡，层层递进，锻炼最强大脑。',
      g3_tag: '策略 · 闯关',
      g3_name: '拯救主公',
      g3_desc: '策略解谜与闯关结合，保护主公过关斩将，节奏紧凑耐玩。',
      g4_tag: '射击 · H5',
      g4_name: '向僵尸开炮',
      g4_desc: '经典射击玩法 H5 化，爽快清屏，支持平台热更新与版本管理。',
      g5_tag: '街机 · 空战',
      g5_name: '雷霆战机',
      g5_desc: '全民打飞机式街机射击，操控战机躲避弹幕、击破敌机。',
      g6_tag: '更多',
      g6_name: '持续更新中',
      g6_desc: '消除宝石、极速跳跃、转盘挑战等更多品类，详见星澜小游戏平台客户端。',
      about_title: '关于星澜无界',
      about_body: '<strong>StarLaneInfinite Technology</strong> 致力于「内容无界、体验有界」——用可靠的多媒体与游戏技术，让优质内容触达更多用户。团队具备 Unity 客户端、Go 服务端与 H5 发行全链路能力，从 SCPlayer 播放到小游戏平台上架，形成完整产品矩阵。',
      about_f1: '<strong>SCPlayer</strong> — Unity 多媒体播放器，面向内容与展示场景',
      about_f2: '<strong>星澜小游戏平台</strong> — 账号、金币、任务、排行榜一体化',
      about_f3: '<strong>原创游戏 IP</strong> — 吸血鬼宿舍、最强大脑、拯救主公等持续运营',
      about_f4: '<strong>技术栈</strong> — Unity · H5 · Go API · 运营 IDIP 后台',
      contact_title: '商务与合作',
      contact_body: '游戏联运、SCPlayer 定制、平台技术对接，欢迎与我们联系。',
      contact_email: '企业邮箱：',
      contact_brand: 'StarLaneInfinite Technology · 星澜无界科技',
      footer_brand: '星澜无界科技'
    },
    en: {
      meta_title: 'StarLaneInfinite Technology',
      meta_desc: 'StarLaneInfinite — SCPlayer multimedia player and casual games platform: Vampire Dormitory, Brain Master, Save the Lord, and more.',
      logo_sub: 'StarLane Infinite Tech',
      nav_products: 'Products',
      nav_games: 'Games',
      nav_about: 'About',
      nav_contact: 'Contact',
      lang_label: 'Language',
      hero_eyebrow: 'Multimedia · Casual Games · Unity & H5',
      hero_title: 'Boundless experiences<br />connecting content & play',
      hero_lead: '<strong>StarLaneInfinite Technology</strong> builds multimedia playback and lightweight gaming ecosystems — <strong>SCPlayer</strong>, our Unity media player, and the <strong>StarLane Mini Games Platform</strong>, featuring Vampire Dormitory, Brain Master, Save the Lord, and more.',
      hero_btn_products: 'Our Products',
      hero_btn_games: 'Browse Games',
      products_title: 'Core Products',
      products_desc: 'From playback to play — one stack for content and operations.',
      sc_tag: 'Multimedia',
      sc_sub: 'Multimedia Player · Unity',
      sc_desc: 'A high-performance Unity-based media player for common audio/video formats and smooth UI — ideal for education, showcases, embedded playback, and the StarLane content ecosystem.',
      sc_p1: 'Unity cross-platform rendering & UI',
      sc_p2: 'Playback-first design with extensible plugins',
      sc_p3: 'Custom deployment for enterprises & content partners',
      plat_tag: 'Platform',
      plat_name: 'StarLane Mini Games Platform',
      plat_sub: 'Unified publishing · H5 hot updates',
      plat_desc: 'Lightweight publishing with unified accounts, rewards, and game catalogs. Clients fetch live game lists; H5 packages update online; ops configure releases, versions, and channels.',
      plat_p1: 'One login & tasks across multiple games',
      plat_p2: 'H5 and Unity hybrid integration',
      plat_p3: 'IDIP ops console & audit trails',
      games_title: 'Featured Mini Games',
      games_desc: 'Original and partner titles on StarLane — instant play, perfect for short sessions.',
      g1_tag: 'Casual · Sim',
      g1_name: 'Vampire Dormitory',
      g1_desc: 'Light dorm-themed casual fun — easy to pick up for all ages.',
      g2_tag: 'Puzzle · Brain',
      g2_name: 'Brain Master',
      g2_desc: 'Levels that test observation and logic — train your brain step by step.',
      g3_tag: 'Strategy · Levels',
      g3_name: 'Save the Lord',
      g3_desc: 'Strategy puzzles meet stage progression — protect the lord and clear each level.',
      g4_tag: 'Shooter · H5',
      g4_name: 'Zombie Blaster',
      g4_desc: 'Classic shooter in H5 — fast action with platform hot updates.',
      g5_tag: 'Arcade · Shooter',
      g5_name: 'Thunder Fighter',
      g5_desc: 'Arcade aerial combat — dodge bullets and destroy enemy aircraft.',
      g6_tag: 'More',
      g6_name: 'Coming Soon',
      g6_desc: 'Gem Crush, Speed Jump, Spin Wheel, and more — see the StarLane client for the full catalog.',
      about_title: 'About StarLane Infinite',
      about_body: '<strong>StarLaneInfinite Technology</strong> stands for boundless content, focused experience — reliable multimedia and game tech to reach more users. Our team covers Unity clients, Go backends, and H5 publishing from SCPlayer to platform launch.',
      about_f1: '<strong>SCPlayer</strong> — Unity multimedia player for content & display',
      about_f2: '<strong>StarLane Mini Games Platform</strong> — accounts, coins, tasks, leaderboards',
      about_f3: '<strong>Original IP</strong> — Vampire Dormitory, Brain Master, Save the Lord & more',
      about_f4: '<strong>Stack</strong> — Unity · H5 · Go API · IDIP ops backend',
      contact_title: 'Business & Partnerships',
      contact_body: 'Game publishing, SCPlayer customization, platform integration — get in touch.',
      contact_email: 'Email: ',
      contact_brand: 'StarLaneInfinite Technology',
      footer_brand: 'StarLane Infinite Tech'
    },
    ur: {
      meta_title: 'StarLaneInfinite Technology',
      meta_desc: 'StarLaneInfinite — SCPlayer ملٹی میڈیا پلیئر اور کیژول گیمز پلیٹ فارم: Vampire Dormitory، Brain Master، Save the Lord وغیرہ۔',
      logo_sub: 'اسٹار لین انفinite ٹیکنالوجی',
      nav_products: 'مصنوعات',
      nav_games: 'گیمز',
      nav_about: 'تعارف',
      nav_contact: 'رابطہ',
      lang_label: 'زبان',
      hero_eyebrow: 'Multimedia · Casual Games · Unity & H5',
      hero_title: 'مواد اور کھیل کو<br />جوڑنے والا لامحدود تجربہ',
      hero_lead: '<strong>StarLaneInfinite Technology</strong> ملٹی میڈیا پلے بیک اور ہلکے گیمز کے ماحولیاتی نظام بناتی ہے — <strong>SCPlayer</strong> Unity پلیئر اور <strong>StarLane Mini Games Platform</strong>، جس میں Vampire Dormitory، Brain Master، Save the Lord وغیرہ شامل ہیں۔',
      hero_btn_products: 'مصنوعات دیکھیں',
      hero_btn_games: 'گیمز دیکھیں',
      products_title: 'اہم مصنوعات',
      products_desc: 'پلے بیک سے کھیل تک — مواد اور آپریشنز کے لیے ایک ٹیکنالوجی اسٹیک۔',
      sc_tag: 'ملٹی میڈیا',
      sc_sub: 'ملٹی میڈیا پلیئر · Unity',
      sc_desc: 'Unity پر مبنی اعلیٰ کارکردگی والا ملٹی میڈیا پلیئر، عام آڈیو/ویڈیو فارمیٹس اور ہموار UI — تعلیم، نمائش اور StarLane مواد کے لیے۔',
      sc_p1: 'Unity کراس پلیٹ فارم رینڈرنگ اور UI',
      sc_p2: 'پلے بیک پر مبنی ڈیزائن، توسیعی پلگ ان',
      sc_p3: 'اداروں اور مواد شراکت داروں کے لیے کسٹم تعینات',
      plat_tag: 'پلیٹ فارم',
      plat_name: 'StarLane Mini Games Platform',
      plat_sub: 'متحد اشاعت · H5 hot update',
      plat_desc: 'متحد اکاؤنٹ، انعامات اور گیمز کی فہرست۔ کلائنٹ لائیو فہرست حاصل کرتا ہے؛ H5 پیکج آن لائن اپ ڈیٹ؛ آپریشنز ریلیز اور چینلز کنفیگر کرتے ہیں۔',
      plat_p1: 'متعدد گیمز میں ایک لاگ ان اور ٹاسks',
      plat_p2: 'H5 / Unity ہائبرڈ انضمام',
      plat_p3: 'IDIP آپریشنز کنسول اور آڈٹ',
      games_title: 'نمایاں منی گیمز',
      games_desc: 'StarLane پر اصل اور پارٹنر عنوانات — فوری کھیل، مختصر سیشنز کے لیے موزوں۔',
      g1_tag: 'کیژول · Sim',
      g1_name: 'Vampire Dormitory',
      g1_desc: 'ہلکی dorm thème کیژول گیم — ہر عمر کے لیے آسان۔',
      g2_tag: 'پزل · دماغ',
      g2_name: 'Brain Master',
      g2_desc: 'مشاہدے اور منطق کی آزمائش — مرحلہ وار دماغ کی مشق۔',
      g3_tag: 'حکمت · مراحل',
      g3_name: 'Save the Lord',
      g3_desc: 'حکمت عملی پزل اور مرحلہ وار پیش رفت — lord کی حفاظت کریں۔',
      g4_tag: 'شوٹر · H5',
      g4_name: 'Zombie Blaster',
      g4_desc: 'کلاسک شوٹر H5 میں — تیز کارروائی، پلیٹ فارم hot update۔',
      g5_tag: 'آرکیڈ · ہواائی',
      g5_name: 'Thunder Fighter',
      g5_desc: 'آرکیڈ ہواائی جنگ — گولیوں سے بچیں، دشمن تباہ کریں۔',
      g6_tag: 'مزید',
      g6_name: 'جلد آ رہا ہے',
      g6_desc: 'Gem Crush، Speed Jump، Spin Wheel وغیرہ — مکمل فہرست StarLane کلائنٹ میں۔',
      about_title: 'StarLane Infinite کے بارے میں',
      about_body: '<strong>StarLaneInfinite Technology</strong> لامحدود مواد، مرکوز تجربہ — قابل اعتماد ملٹی میڈیا اور گیم ٹیکنالوجی۔ Unity کلائنٹ، Go بیک اینڈ، H5 اشاعت — SCPlayer سے پلیٹ فارم لانچ تک۔',
      about_f1: '<strong>SCPlayer</strong> — Unity ملٹی میڈیا پلیئر',
      about_f2: '<strong>StarLane Mini Games Platform</strong> — اکاؤنٹ، سکے، ٹاسks، لیڈر بورڈ',
      about_f3: '<strong>اصل IP</strong> — Vampire Dormitory، Brain Master، Save the Lord وغیرہ',
      about_f4: '<strong>اسٹیک</strong> — Unity · H5 · Go API · IDIP ops',
      contact_title: 'کاروبار اور شراکت داری',
      contact_body: 'گیم اشاعت، SCPlayer customization، پلیٹ فارم انضمام — رابطہ کریں۔',
      contact_email: 'ای میل: ',
      contact_brand: 'StarLaneInfinite Technology',
      footer_brand: 'StarLane Infinite Tech'
    }
  };

  function getLang() {
    var saved = localStorage.getItem(STORAGE_KEY);
    if (saved && I18N[saved]) return saved;
    var nav = (navigator.language || '').toLowerCase();
    if (nav.indexOf('ur') === 0) return 'ur';
    if (nav.indexOf('en') === 0) return 'en';
    return DEFAULT_LANG;
  }

  function t(lang, key) {
    var pack = I18N[lang] || I18N[DEFAULT_LANG];
    return pack[key] != null ? pack[key] : (I18N[DEFAULT_LANG][key] || key);
  }

  var LANG_LABELS = { zh: '中文', en: 'English', ur: 'اردو' };

  function updateLangPickerUI(lang) {
    var current = document.getElementById('lang-picker-current');
    if (current) current.textContent = LANG_LABELS[lang] || lang;
    document.querySelectorAll('.lang-picker-menu [data-lang]').forEach(function (li) {
      var on = li.getAttribute('data-lang') === lang;
      li.classList.toggle('is-selected', on);
      li.setAttribute('aria-selected', on ? 'true' : 'false');
    });
  }

  function initLangPicker() {
    var btn = document.getElementById('lang-picker-btn');
    var menu = document.getElementById('lang-picker-menu');
    if (!btn || !menu) return;

    btn.addEventListener('click', function (e) {
      e.stopPropagation();
      var open = menu.hidden;
      menu.hidden = !open;
      btn.setAttribute('aria-expanded', open ? 'true' : 'false');
    });

    menu.querySelectorAll('[data-lang]').forEach(function (li) {
      li.addEventListener('click', function (e) {
        e.stopPropagation();
        applyLang(li.getAttribute('data-lang'));
        menu.hidden = true;
        btn.setAttribute('aria-expanded', 'false');
      });
    });

    document.addEventListener('click', function () {
      menu.hidden = true;
      btn.setAttribute('aria-expanded', 'false');
    });
  }

  function applyLang(lang) {
    if (!I18N[lang]) lang = DEFAULT_LANG;
    localStorage.setItem(STORAGE_KEY, lang);

    document.documentElement.lang = lang === 'zh' ? 'zh-CN' : lang === 'ur' ? 'ur' : 'en';
    document.documentElement.dir = lang === 'ur' ? 'rtl' : 'ltr';

    document.title = t(lang, 'meta_title');
    var metaDesc = document.querySelector('meta[name="description"]');
    if (metaDesc) metaDesc.setAttribute('content', t(lang, 'meta_desc'));

    document.querySelectorAll('[data-i18n]').forEach(function (el) {
      el.textContent = t(lang, el.getAttribute('data-i18n'));
    });
    document.querySelectorAll('[data-i18n-html]').forEach(function (el) {
      el.innerHTML = t(lang, el.getAttribute('data-i18n-html'));
    });

    updateLangPickerUI(lang);
  }

  document.addEventListener('DOMContentLoaded', function () {
    initLangPicker();
    applyLang(getLang());
  });
})();
