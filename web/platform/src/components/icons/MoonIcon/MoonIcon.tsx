import { assetPaths } from "@/assets/asset-paths";

import { AssetIcon, type AssetIconProps } from "../AssetIcon";

type MoonIconProps = Omit<AssetIconProps, "iconName" | "source">;

export function MoonIcon(props: Readonly<MoonIconProps>) {
  return <AssetIcon {...props} iconName="moon" source={assetPaths.icons.theme.moon} />;
}
