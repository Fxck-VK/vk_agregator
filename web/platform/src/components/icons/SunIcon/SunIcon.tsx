import { assetPaths } from "@/assets/asset-paths";

import { AssetIcon, type AssetIconProps } from "../AssetIcon";

type SunIconProps = Omit<AssetIconProps, "iconName" | "source">;

export function SunIcon(props: Readonly<SunIconProps>) {
  return <AssetIcon {...props} iconName="sun" source={assetPaths.icons.theme.sun} />;
}
