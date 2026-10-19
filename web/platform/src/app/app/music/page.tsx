import { MusicWorkspaceController } from "@/features/music/MusicWorkspace";
import { parseMusicContinuation } from "@/features/music/music-navigation";

export default async function MusicPage({ searchParams }: { searchParams: Promise<Record<string, string | string[] | undefined>> }) {
  const params = await searchParams;
  return <MusicWorkspaceController requestedModelId={typeof params.model === "string" ? params.model : undefined}
    requestedContinuation={parseMusicContinuation(params)} />;
}
