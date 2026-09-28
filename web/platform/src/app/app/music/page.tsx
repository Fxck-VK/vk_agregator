import { MusicWorkspaceController } from "@/features/music/MusicWorkspace";

export default async function MusicPage({ searchParams }: { searchParams: Promise<{ model?: string | string[] }> }) {
  const { model } = await searchParams;
  return <MusicWorkspaceController requestedModelId={typeof model === "string" ? model : undefined} />;
}
