import {
  modelSearchAliases as aliases,
  modelSearchAlphabet,
  modelSearchKeyboard,
  modelSearchLetterFolds,
  modelSearchPhrases,
} from "@/i18n/model-search-data";

type SearchableModel = { id: string; name: string };

const { cyrillic, latin } = modelSearchAlphabet;
const transliteration = new Map(Array.from(cyrillic, (letter, index) => [letter, latin[index]]));
const keyboard = new Map<string, string>();
Array.from(modelSearchKeyboard.latin).forEach((letter, index) => {
  keyboard.set(letter, modelSearchKeyboard.russian[index]);
  keyboard.set(modelSearchKeyboard.russian[index], letter);
});

function normalize(value: string) {
  let folded = value.normalize("NFKC").toLowerCase();
  for (const [from, to] of Object.entries(modelSearchLetterFolds)) folded = folded.replaceAll(from, to);
  return folded
    .replace(/(\p{L})(\p{N})/gu, "$1 $2").replace(/(\p{N})(\p{L})/gu, "$1 $2")
    .replace(/[^\p{L}\p{N}]+/gu, " ").trim();
}

function romanize(value: string) {
  for (const [from, to] of Object.entries(modelSearchPhrases)) value = value.replaceAll(from, to);
  return value.split(" ")
    .map(word => Object.hasOwn(aliases, word) ? aliases[word] : Array.from(word, letter => transliteration.get(letter) ?? letter).join(""))
    .join(" ");
}

function queryVariants(query: string) {
  const original = normalize(query);
  const swapped = normalize(Array.from(query.toLowerCase(), letter => keyboard.get(letter) ?? letter).join(""));
  const variants = new Map<string, number>();
  for (const [text, penalty] of [[original, 0], [romanize(original), 1], [swapped, 2], [romanize(swapped), 3]] as const) {
    if (text && !variants.has(text)) variants.set(text, penalty);
  }
  return [...variants].map(([text, penalty]) => ({ text, penalty }));
}

// Bounded edit distance with adjacent transpositions (e.g. banaan -> banana).
function editDistance(left: string, right: string, limit: number) {
  if (Math.abs(left.length - right.length) > limit) return limit + 1;
  let previous = Array.from({ length: right.length + 1 }, (_, index) => index);
  let beforePrevious = previous;
  for (let row = 1; row <= left.length; row++) {
    const current = [row];
    for (let column = 1; column <= right.length; column++) {
      current[column] = Math.min(
        previous[column] + 1, current[column - 1] + 1,
        previous[column - 1] + Number(left[row - 1] !== right[column - 1]),
      );
      if (row > 1 && column > 1 && left[row - 1] === right[column - 2] && left[row - 2] === right[column - 1]) {
        current[column] = Math.min(current[column], beforePrevious[column - 2] + 1);
      }
    }
    if (Math.min(...current) > limit) return limit + 1;
    beforePrevious = previous;
    previous = current;
  }
  return previous[right.length];
}

function wordCost(query: string, target: string) {
  if (query === target) return 0;
  // Version digits are exact: searching 5.5 must not suggest 5.6.
  if (/\p{N}/u.test(query) || /\p{N}/u.test(target)) return Infinity;
  if (query.length >= 2 && target.startsWith(query)) return 1;
  if (query.length < 4 || target.length < 4) return Infinity;
  const limit = Math.min(2, Math.floor(query.length / 4));
  const distance = editDistance(query, target, limit);
  return distance <= limit ? 4 + distance : Infinity;
}

function fieldScore(field: string, query: string) {
  const numbers = query.match(/\p{N}+/gu);
  if (numbers && !` ${(field.match(/\p{N}+/gu) ?? []).join(" ")} `.includes(` ${numbers.join(" ")} `)) return Infinity;
  if (field === query || field.replaceAll(" ", "") === query.replaceAll(" ", "")) return 0;
  const words = field.split(" ");
  const queryWords = query.split(" ").flatMap(word => {
    if (words.includes(word)) return [word];
    for (let start = 0; start < words.length; start++) {
      for (let end = start + 2; end <= words.length; end++) {
        if (words.slice(start, end).join("") === word) return words.slice(start, end);
      }
    }
    return [word];
  });
  if (queryWords.length > words.length) return Infinity;
  const used = new Set<number>();
  let cost = 0;
  // Match exact/numeric terms first so a partial token cannot consume their slot.
  const ordered = [...queryWords].sort((a, b) => Number(words.includes(b)) - Number(words.includes(a)) || b.length - a.length);
  for (const word of ordered) {
    let bestCost = Infinity;
    let bestIndex = -1;
    words.forEach((target, index) => {
      if (used.has(index)) return;
      const candidate = wordCost(word, target);
      if (candidate < bestCost) { bestCost = candidate; bestIndex = index; }
    });
    if (bestIndex === -1) {
      // Joined spellings such as nanobanana can span adjacent name words.
      if (/^[a-z]{4,}$/u.test(word) && field.replaceAll(" ", "").includes(word) && queryWords.length === 1) return 22;
      return Infinity;
    }
    used.add(bestIndex);
    cost += bestCost;
  }
  return 20 + cost * 10 + Math.min(words.length - queryWords.length, 9);
}

/** Search only the supplied available models; keep objects, stable ties and exact versions. */
export function searchModels<T extends SearchableModel>(models: readonly T[], query: string): T[] {
  if (!query.trim()) return [...models];
  if (query.length > 200) return [];
  const variants = queryVariants(query);
  const seen = new Set<string>();
  return models.flatMap((model, index) => {
    if (seen.has(model.id)) return [];
    seen.add(model.id);
    const fields = [normalize(model.name), normalize(model.id)];
    const score = Math.min(...variants.flatMap(({ text, penalty }) => fields.map(field => fieldScore(field, text) + penalty)));
    return Number.isFinite(score) ? [{ model, index, score }] : [];
  }).sort((left, right) => left.score - right.score || left.index - right.index)
    .map(({ model }) => model);
}
