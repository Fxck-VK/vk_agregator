"use client";

import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { useMessages } from "@/i18n/LocaleProvider";

import Markdown from "react-markdown";

import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import styles from "./AssistantMessageContent.module.css";

type AssistantMessageContentProps = {
  markdown: string;
  omitImageArtifactIDs?: readonly string[];
};

const imageArtifactPathPattern = /^\/web\/v1\/image-artifacts\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const allowedElements = [
  "p",
  "h1",
  "h2",
  "h3",
  "h4",
  "ol",
  "ul",
  "li",
  "strong",
  "em",
  "blockquote",
  "pre",
  "code",
  "a",
  "hr",
  "br",
  "img",
];

export function AssistantMessageContent({ markdown, omitImageArtifactIDs = [] }: Readonly<AssistantMessageContentProps>) {
  const msg = useMessages();
  const omittedPaths = new Set(omitImageArtifactIDs.map((id) => `/web/v1/image-artifacts/${id}`));
  const safeMarkdown = markdown
    .replace(/<script\b[^>]*>[\s\S]*?<\/script\s*>/gi, "")
    .replace(/<style\b[^>]*>[\s\S]*?<\/style\s*>/gi, "");

  return (
    <div className={styles.content} dir="auto">
      <Markdown
        allowedElements={allowedElements}
        components={{
          a({ node, href, ...props }) {
            void node;
            if (href && omittedPaths.has(href)) return null;
            const isImageDownload = typeof href === "string" && imageArtifactPathPattern.test(href);
            return <a {...props} download={isImageDownload || undefined} href={href} rel="noopener noreferrer" target={isImageDownload ? undefined : "_blank"} />;
          },
          img({ src, alt }) {
            if (typeof src !== "string" || !imageArtifactPathPattern.test(src)) return null;
            if (omittedPaths.has(src)) return null;
            return (
              <MediaImage alt={alt || msg("assistantMessageContent.generatedImage")} className={styles.image} decoding="async" loading="lazy" src={src} />
            );
          },
          pre({ children }) {
            return (
              <ScrollArea
                className={styles.codeScroll}
                orientation="horizontal"
                viewportAs="pre"
                viewportProps={{ dir: "ltr" }}
                viewportClassName={styles.codeBlock}
              >
                {children}
              </ScrollArea>
            );
          },
        }}
        skipHtml
      >
        {safeMarkdown}
      </Markdown>
    </div>
  );
}
