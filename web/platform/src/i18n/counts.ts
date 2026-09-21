import type { MessageKey, Translator } from "./messages";
import { formatNumber } from "./format";

export function countLabel(msg: Translator, unit: "tokens" | "files" | "outputTokens" | "stars" | "actionStars", count: number): string {
  const form = new Intl.PluralRules(msg.locale).select(count);
  const key: MessageKey = `units.${unit}.${form}`;
  return msg(key, { count: formatNumber(msg.locale, count) });
}

export function tokenAmount(msg: Translator, count: number | undefined, unit: "tokens" | "outputTokens" = "tokens"): string {
  return count === undefined ? "—" : countLabel(msg, unit, count);
}

export function actionCreditAmount(msg: Translator, value: number): string {
  return countLabel(msg, "actionStars", value);
}
