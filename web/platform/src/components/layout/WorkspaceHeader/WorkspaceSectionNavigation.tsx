"use client";

import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { useDictionary, useMessages } from "@/i18n/LocaleProvider";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";
import { usePathname, useRouter } from "@/i18n/navigation";

import styles from "./WorkspaceHeader.module.css";

export function WorkspaceSectionNavigation() {
  const pathname = usePathname();
  const router = useRouter();
  const t = useDictionary();
  const msg = useMessages();
  const { allowsPath } = useReleaseView();
  const items = [
    { id: "/app/music", label: t.navigation.music },
    { id: "/app/speech", label: t.navigation.speech },
    { id: "/app/audio-download", label: msg("audioDownload.title") },
  ].filter(item => allowsPath(item.id));
  if (items.length < 2 || !items.some(item => item.id === pathname)) return null;

  return (
    <ModeSwitchPanel
      activeID={pathname}
      ariaLabel={t.navigation.audio}
      className={styles.sectionNavigation}
      density="compact"
      items={items}
      onChange={path => { if (path !== pathname) router.push(path); }}
    />
  );
}
