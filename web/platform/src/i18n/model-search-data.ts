// Input vocabulary, independent of the selected UI locale. Never catalog entries.
export const modelSearchAliases: Readonly<Record<string, string>> = {
  нано: "nano", нана: "nano", банана: "banana", нанобанана: "nano banana",
  нанабанана: "nano banana", про: "pro", лайт: "lite", мини: "mini",
  макс: "max", флеш: "flash", фаст: "fast", турбо: "turbo",
  сидрим: "seedream", сидреам: "seedream", клинг: "kling",
  гпт: "gpt", джипити: "gpt", чатгпт: "chatgpt",
  джемини: "gemini", гемини: "gemini", клод: "claude", клауд: "claude",
  опус: "opus", соннет: "sonnet", хайку: "haiku",
  миджорни: "midjourney", миджерни: "midjourney",
  дипсик: "deepseek", квен: "qwen", куэн: "qwen",
  флюкс: "flux", флакс: "flux", грок: "grok", вео: "veo",
  суно: "suno", лирия: "lyria", виспер: "whisper", уиспер: "whisper",
  сиданс: "seedance", сиденс: "seedance", ранвей: "runway", ранвэй: "runway",
  минимакс: "minimax", виду: "vidu", ван: "wan", пиксверс: "pixverse",
  кими: "kimi", имаген: "imagen", имаджен: "imagen", имейдж: "image",
  хэппихорс: "happyhorse", скайрилс: "skyreels",
};

export const modelSearchAlphabet = {
  cyrillic: "абвгдежзийклмнопрстуфхцчшщъыьэюя",
  latin: ["a", "b", "v", "g", "d", "e", "zh", "z", "i", "i", "k", "l", "m", "n", "o", "p", "r", "s", "t", "u", "f", "kh", "ts", "ch", "sh", "shch", "", "y", "", "e", "yu", "ya"],
};
export const modelSearchKeyboard = {
  latin: "qwertyuiop[]asdfghjkl;'zxcvbnm,.`",
  russian: "йцукенгшщзхъфывапролджэячсмитьбюё",
};
export const modelSearchLetterFolds: Readonly<Record<string, string>> = { ё: "е" };
export const modelSearchPhrases: Readonly<Record<string, string>> = { "джи пи ти": "gpt" };
