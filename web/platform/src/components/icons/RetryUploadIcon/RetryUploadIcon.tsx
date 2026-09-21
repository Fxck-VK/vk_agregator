import { assetPaths } from "@/assets/asset-paths";

import { AssetIcon, type AssetIconProps } from "../AssetIcon";

export type RetryUploadIconProps = Omit<AssetIconProps, "iconName" | "source">;

export function RetryUploadIcon(props: Readonly<RetryUploadIconProps>) {
  return <AssetIcon {...props} iconName="retry-upload" source={assetPaths.icons.ui.retryUpload} />;
}
