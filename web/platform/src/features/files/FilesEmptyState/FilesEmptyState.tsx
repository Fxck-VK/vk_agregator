import Image from "next/image";

import { assetPaths } from "@/assets/asset-paths";

import styles from "./FilesEmptyState.module.css";

type FilesEmptyStateProps = {
  description: string;
  title: string;
};

export function FilesEmptyState({ description, title }: Readonly<FilesEmptyStateProps>) {
  return (
    <section className={styles.emptyState}>
      <Image
        alt=""
        aria-hidden="true"
        className={styles.folder}
        height={1254}
        src={assetPaths.illustrations.filesEmptyFolder}
        width={1254}
      />
      <div>
        <h2>{title}</h2>
        <p>{description}</p>
      </div>
    </section>
  );
}
