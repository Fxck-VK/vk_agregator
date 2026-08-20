import { assetPaths } from "@/assets/asset-paths";

import { AssetIcon, type AssetIconProps } from "../AssetIcon";

type ProfileIconProps = Omit<AssetIconProps, "iconName" | "source">;

export function ProfileIcon(props: Readonly<ProfileIconProps>) {
  return <AssetIcon {...props} iconName="profile" source={assetPaths.icons.accountMenu.profile} />;
}
