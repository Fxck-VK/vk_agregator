export const ru = {
  document: {
    title: "NeiroHub — рабочее пространство",
    description: "Единая рабочая среда для нейросетевых инструментов.",
  },
  brand: {
    name: "NeiroHub",
    monogram: "NH",
  },
  home: {
    title: "Нейросети в одном рабочем пространстве",
    description: "Откройте рабочую область, чтобы выбрать инструмент и начать новую задачу.",
    primaryAction: "Открыть рабочее пространство",
    supportingText: "Новый интерфейс развивается поэтапно: без переноса личных данных в публичную страницу.",
  },
  navigation: {
    regionLabel: "Навигационная панель",
    label: "Основная навигация",
    openMenuLabel: "Открыть меню",
    closeMenuLabel: "Закрыть меню",
    workspace: "Рабочее пространство",
    chats: "Новый чат",
    files: "Мои файлы",
    models: "Все нейросети",
    inspiration: "Вдохновение",
  },
  accountPreview: {
    label: "Предпросмотр аккаунта",
    title: "Аккаунт",
    description: "Настройки появятся здесь",
  },
  login: {
    emailLabel: "Электронная почта",
    passwordLabel: "Пароль",
    submitLabel: "Войти",
    pending: "Входим…",
    failure: "Не удалось войти. Попробуйте ещё раз.",
  },
  account: {
    heading: "Аккаунт",
    unavailableLabel: "Данные аккаунта недоступны",
    logoutLabel: "Выйти",
    logoutPending: "Выходим…",
    logoutFailure: "Не удалось выйти. Попробуйте ещё раз.",
  },
  conversations: {
    recentHeading: "Недавние чаты",
    empty: "Недавних чатов пока нет.",
    unnamed: "Без названия",
    createLabel: "Создать чат",
    createPending: "Создаём чат…",
    createFailure: "Не удалось создать чат. Попробуйте ещё раз.",
  },
  workspace: {
    eyebrow: "NeiroHub",
    title: "Рабочее пространство",
    description: "Выберите раздел, чтобы начать работу.",
    quickStartTitle: "С чего начать",
    quickStartDescription: "Разделы уже готовы как безопасная основа интерфейса. Подключение личных данных и рабочих сценариев будет отдельным шагом.",
    openChats: "Открыть чаты",
    openModels: "Посмотреть нейросети",
    unavailable: "Рабочее пространство временно недоступно. Попробуйте ещё раз.",
    refreshPending: "Восстанавливаем сессию…",
    chatPlaceholder: "Чат готовится к работе.",
    sections: {
      home: {
        title: "Рабочее пространство",
        description: "Выберите направление для следующей задачи.",
      },
      chats: {
        title: "Чаты",
        description: "Здесь появится личная история диалогов после подключения сессии.",
      },
      files: {
        title: "Мои файлы",
        description: "Здесь будут отображаться только файлы, доступ к которым подтвердил сервер.",
      },
      models: {
        title: "Все нейросети",
        description: "Каталог инструментов появится в следующем функциональном срезе.",
      },
      inspiration: {
        title: "Вдохновение",
        description: "Подборки идей и примеров будут опубликованы здесь отдельно от личного пространства.",
      },
    },
  },
} as const;
