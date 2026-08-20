import { assetPaths } from "@/assets/asset-paths";

import { AssetIcon, type AssetIconProps } from "../AssetIcon";

type SupportIconProps = Omit<AssetIconProps, "iconName" | "source">;

export function SupportIcon(props: Readonly<SupportIconProps>) {
  return <AssetIcon {...props} iconName="support" source={assetPaths.icons.accountMenu.support} />;
}
