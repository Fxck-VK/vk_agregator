import { FilesWorkspace } from "@/features/files/FilesWorkspace/FilesWorkspace";
import type { FileCategory } from "@/features/files/FileTypeTabs/FileTypeTabs";

type FilesPageProps = {
  searchParams: Promise<{ category?: string | string[] }>;
};

const fileCategories = new Set<FileCategory>(["all", "images", "reports", "presentations", "video", "uploads"]);

function requestedFileCategory(value: string | string[] | undefined): FileCategory {
  const category = Array.isArray(value) ? value[0] : value;
  return category !== undefined && fileCategories.has(category as FileCategory)
    ? category as FileCategory
    : "all";
}

export default async function FilesPage({ searchParams }: Readonly<FilesPageProps>) {
  const { category } = await searchParams;
  return <FilesWorkspace initialCategory={requestedFileCategory(category)} />;
}
