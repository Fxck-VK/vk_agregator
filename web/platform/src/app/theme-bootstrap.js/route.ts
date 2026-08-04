import { themeBootstrapScript } from "@/features/theme/theme-preference";

export const dynamic = "force-static";

export function GET(): Response {
  return new Response(themeBootstrapScript, {
    headers: {
      "Cache-Control": "public, max-age=3600, stale-while-revalidate=86400",
      "Content-Type": "text/javascript; charset=utf-8",
    },
  });
}
