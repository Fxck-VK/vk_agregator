import "server-only";
import { mkdir, readFile, rename, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { isLocalWorkspacePreviewEnabled } from "@/features/session/local-workspace-preview";

const directory = join(process.cwd(), ".cache");
const path = join(directory, "local-preview-deleted-artifacts.json");
let writes: Promise<void> = Promise.resolve();
export async function previewDeletedArtifactIDs(): Promise<readonly string[]> {
  if (!isLocalWorkspacePreviewEnabled()) return [];
  try {
    const value: unknown = JSON.parse(await readFile(path, "utf8"));
    if (!Array.isArray(value) || value.length > 10_000 || value.some(id => typeof id !== "string" || !/^[a-f0-9-]{36}$/i.test(id))) throw new Error("Invalid preview deletion state");
    return value;
  } catch (error) { if ((error as NodeJS.ErrnoException).code === "ENOENT") return []; throw error; }
}
export async function markPreviewArtifactDeleted(id: string) {
  if (!isLocalWorkspacePreviewEnabled()) throw new Error("Preview disabled");
  const write = writes.catch(() => undefined).then(async () => {
    const ids = new Set(await previewDeletedArtifactIDs());
    ids.add(id);
    await mkdir(directory, { recursive: true });
    const temporary = `${path}.${crypto.randomUUID()}.tmp`;
    await writeFile(temporary, JSON.stringify([...ids]));
    await rename(temporary, path);
  });
  writes = write;
  await write;
}
export async function previewWithTombstones<T>(value: T): Promise<T> {
  const deleted = new Set(await previewDeletedArtifactIDs());
  if (!deleted.size) return value;
  const visit = (input: unknown): unknown => {
    if (Array.isArray(input)) return input.map(visit);
    if (!input || typeof input !== "object") return input;
    const result = Object.fromEntries(Object.entries(input).map(([key, entry]) => [key, visit(entry)]));
    if (typeof result.id === "string" && deleted.has(result.id) && typeof result.mime_type === "string") { result.deleted = true; if ("size_bytes" in result) result.size_bytes = 0; }
    return result;
  };
  return visit(value) as T;
}
