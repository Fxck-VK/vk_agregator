import { assetPaths } from "@/assets/asset-paths";

import { AssetIcon, type AssetIconProps } from "../AssetIcon";

export type MusicIconProps = Omit<AssetIconProps, "iconName" | "source">;

export function MusicIcon(props: Readonly<MusicIconProps>) {
  return <AssetIcon {...props} iconName="music" source={assetPaths.icons.ui.music} />;
}
