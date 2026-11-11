// This comparison is private and Russian-only, including legacy dictionary fallback.
const heroAnimation = {
  title: 'Анимация текста',
  options: [
    { id: 'fade', label: 'Смена', description: 'Фразы плавно сменяются через растворение и лёгкое размытие.' },
    { id: 'words', label: 'По словам', description: 'Каждая фраза раскрывается последовательно, слово за словом.' },
    { id: 'spotlight', label: 'Подсветка', description: 'Фразы остаются на месте, а световой акцент переходит между ними.' },
  ],
} as const;

export const landingVariantsText = {
  ru: {
    title: 'Варианты главной',
    description: 'Две исходные версии и отдельный третий вариант для дальнейшей доработки.',
    original: 'Наша главная',
    originalDescription: 'Исходное оформление и содержимое SEO-шаблона T01.',
    friend: 'Версия друга',
    friendDescription: 'Главная из dev-deploy, коммит ba2001cd. Отдельные стили, тексты и изображения.',
    third: 'Третий вариант',
    thirdDescription: 'Наша вёрстка и тексты с фотографиями из версии друга. Отдельная страница и стили.',
    open: 'Открыть →',
    back: 'Вернуться в платформу',
    heroAnimation,
  },
  en: {
    title: 'Homepage variants',
    description: 'Two preserved source versions and a separate third variant for further work.',
    original: 'Our homepage',
    originalDescription: 'Original presentation and content of the T01 SEO template.',
    friend: 'Contributor version',
    friendDescription: 'Homepage from dev-deploy, commit ba2001cd. Separate styles, copy and images.',
    third: 'Third variant',
    thirdDescription: 'Our layout and copy with photographs from the contributor version. Separate page and styles.',
    open: 'Open →',
    back: 'Return to workspace',
    heroAnimation,
  },
} as const;
