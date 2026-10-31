"use client";

import { ModeSwitchPanel } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { useDictionary } from "@/i18n/LocaleProvider";
import { usePathname, useRouter } from "@/i18n/navigation";

import styles from "./WorkspaceHeader.module.css";

export function WorkspaceSectionNavigation() {
  const pathname = usePathname();
  const router = useRouter();
  const t = useDictionary();
  if (pathname !== "/app/music" && pathname !== "/app/speech") return null;

  return (
    <ModeSwitchPanel
      activeID={pathname}
      ariaLabel={t.navigation.audio}
      className={styles.sectionNavigation}
      density="compact"
      items={[
        { id: "/app/music", label: t.navigation.music },
        { id: "/app/speech", label: t.navigation.speech },
      ]}
      onChange={path => { if (path !== pathname) router.push(path); }}
    />
  );
}
