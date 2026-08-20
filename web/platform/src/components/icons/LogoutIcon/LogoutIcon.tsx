import { assetPaths } from "@/assets/asset-paths";

import { AssetIcon, type AssetIconProps } from "../AssetIcon";

type LogoutIconProps = Omit<AssetIconProps, "iconName" | "source">;

export function LogoutIcon(props: Readonly<LogoutIconProps>) {
  return <AssetIcon {...props} iconName="logout" source={assetPaths.icons.accountMenu.logout} />;
}
