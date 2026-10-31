import { connection } from "next/server";
import { contentMetadata, renderContentPage } from "@/content/templates/render";

export async function generateMetadata() {
  return contentMetadata("/");
}

export default async function HomePage() {
  await connection();
  return renderContentPage("/");
}
