import { isPasswordTooLong } from "./password";
import commonPatterns from "./password-patterns.json";

// Language-independent lookup data, never shown as UI copy.
const common = new Set(commonPatterns);

export type PasswordStrength = 0 | 1 | 2 | 3;

/** Local UI guidance, not an entropy guarantee or a replacement for server validation. */
export function passwordStrength(password: string, email = ""): PasswordStrength {
  if (!password) return 0;
  if (password.length < 8 || !password.trim() || isPasswordTooLong(password)) return 1;
  // Normalize only the estimate; never modify the value sent to authentication.
  const normalized = password.normalize("NFKC").toLowerCase();
  const letters = normalized.replace(/[^\p{L}]/gu, "");
  const owner = email.split("@")[0].toLowerCase().replace(/[^\p{L}]/gu, "");
  const unique = new Set(normalized).size;
  if (common.has(letters) || /^(.[\s\S]{0,3})\1{2,}$/u.test(normalized) || unique <= 3
    || "01234567890123456789".includes(normalized) || "abcdefghijklmnopqrstuvwxyz".includes(normalized)
    || (owner.length >= 3 && letters === owner)) return 1;
  const classes = [/\p{Ll}/u, /\p{Lu}/u, /\p{N}/u, /[^\p{L}\p{N}\s]/u].filter(pattern => pattern.test(password)).length;
  if ((password.length >= 12 && classes >= 3 && unique >= 8) || (password.length >= 18 && unique >= 8)) return 3;
  if ((password.length >= 10 && classes >= 2 && unique >= 5) || (password.length >= 12 && unique >= 5)) return 2;
  return 1;
}
