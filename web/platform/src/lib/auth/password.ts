// The shared Account Layer limits password hashes to 256 UTF-8 input bytes.
export function isPasswordTooLong(password: string): boolean {
  return new TextEncoder().encode(password).byteLength > 256;
}
