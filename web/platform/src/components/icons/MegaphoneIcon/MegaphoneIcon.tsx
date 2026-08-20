import { assetPaths } from "@/assets/asset-paths";

import { AssetIcon, type AssetIconProps } from "../AssetIcon";

type MegaphoneIconProps = Omit<AssetIconProps, "iconName" | "source">;

export function MegaphoneIcon(props: Readonly<MegaphoneIconProps>) {
  return <AssetIcon {...props} iconName="megaphone" source={assetPaths.icons.accountMenu.megaphone} />;
}
