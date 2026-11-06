"use client";

import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { useDictionary } from "@/i18n/LocaleProvider";
import { useReleaseView } from "@/features/release/ReleaseViewProvider";
import { usePathname, useRouter } from "@/i18n/navigation";

import styles from "./WorkspaceHeader.module.css";

export function WorkspaceSectionNavigation() {
  const pathname = usePathname();
  const router = useRouter();
  const t = useDictionary();
  const { allowsPath } = useReleaseView();
  const items = [
    { id: "/app/music", label: t.navigation.music },
    { id: "/app/speech", label: t.navigation.speech },
  ].filter(item => allowsPath(item.id));
  if (!items.some(item => item.id === pathname)) return null;

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
