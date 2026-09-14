package providermodels

func textCapabilities(m TextAlias) *ModelCapabilities {
	return &ModelCapabilities{
		SchemaVersion: 1,
		API: CapabilityProfile{
			Text:  textAPICapabilities(m),
			Notes: textAPINotes(m),
		},
		Application: CapabilityProfile{
			Text: &TextCapabilities{
				Images: noInput(),
				Videos: noInput(),
				Files:  noInput(),
			},
			Notes: []string{
				"Сейчас можно отправить только текст. Фото, видео, аудио, PDF и другие файлы в запрос не передаются.",
			},
		},
	}
}

func textAPICapabilities(m TextAlias) *TextCapabilities {
	switch m.PublicID {
	case PublicTextGPT55, PublicTextGPT56Terra, PublicTextGPT6Astra:
		return &TextCapabilities{
			Images: supportedInputWithUnlistedExtensions(),
			Videos: unknownInput(),
			Files:  supportedInputWithUnlistedExtensions(),
		}
	case PublicTextGemini31Pro, PublicTextGemini37Flash, PublicTextGemini36Flash:
		return &TextCapabilities{
			Images: supportedInputWithUnlistedExtensions(),
			Videos: supportedInputWithUnlistedExtensions(),
			Files:  inputCapability(Supported, nil, "pdf"),
		}
	default:
		return &TextCapabilities{
			Images: unknownInput(),
			Videos: unknownInput(),
			Files:  unknownInput(),
		}
	}
}

func textAPINotes(m TextAlias) []string {
	switch m.PublicID {
	case PublicTextGPT55, PublicTextGPT56Terra, PublicTextGPT6Astra:
		return []string{
			"Публичная документация для этой модели подтверждает мультимодальный API-ввод с текстом, изображениями и файлами, но не перечисляет точные расширения, лимиты и поддержку видео.",
			"В приложении пока доступна отправка только текста.",
		}
	case PublicTextGemini31Pro, PublicTextGemini37Flash, PublicTextGemini36Flash:
		return []string{
			"Документация подтверждает фото, видео, аудио, PDF и другие документы. PDF указан в списке форматов; полный перечень расширений и лимиты пока не подтверждены.",
			"В приложении пока доступна отправка только текста.",
		}
	case PublicTextClaudeOpus47, PublicTextClaudeOpus48, PublicTextClaudeOpus5, PublicTextClaudeFable5:
		return []string{
			"Для этой версии модели документация провайдера не подтверждает возможности работы с вложениями. Это не означает, что модель их не поддерживает.",
		}
	case PublicTextClaudeFable51:
		return []string{
			"Для этой версии модели документация провайдера не подтверждает возможности работы с вложениями. Это не означает, что модель их не поддерживает.",
		}
	case PublicTextChatGPT:
		return []string{
			"Поддержка вложений у провайдера для базовой модели ещё не подтверждена. В приложении можно отправить только текст.",
		}
	default:
		return []string{
			"Возможности работы с вложениями у провайдера пока не подтверждены.",
		}
	}
}

func supportedInputWithUnlistedExtensions() InputCapability {
	return inputCapability(Supported, nil)
}
