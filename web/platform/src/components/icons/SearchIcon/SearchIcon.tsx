import Image from "next/image";

import { assetPaths } from "@/assets/asset-paths";

type SearchIconProps = {
  className?: string;
};

export function SearchIcon({ className }: Readonly<SearchIconProps>) {
  return (
    <Image
      alt=""
      aria-hidden="true"
      className={className}
      data-icon="search"
      height={24}
      src={assetPaths.icons.ui.search}
      unoptimized
      width={24}
    />
  );
}
