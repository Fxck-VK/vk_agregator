import { assetPaths } from "@/assets/asset-paths";

import { AssetIcon, type AssetIconProps } from "../AssetIcon";

type MonitorIconProps = Omit<AssetIconProps, "iconName" | "source">;

export function MonitorIcon(props: Readonly<MonitorIconProps>) {
  return <AssetIcon {...props} iconName="monitor" source={assetPaths.icons.theme.monitor} />;
}
