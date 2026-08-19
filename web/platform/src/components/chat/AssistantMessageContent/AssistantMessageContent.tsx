import Markdown from "react-markdown";

import styles from "./AssistantMessageContent.module.css";

type AssistantMessageContentProps = {
  markdown: string;
};

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
];

export function AssistantMessageContent({ markdown }: Readonly<AssistantMessageContentProps>) {
  const safeMarkdown = markdown
    .replace(/<script\b[^>]*>[\s\S]*?<\/script\s*>/gi, "")
    .replace(/<style\b[^>]*>[\s\S]*?<\/style\s*>/gi, "");

  return (
    <div className={styles.content}>
      <Markdown
        allowedElements={allowedElements}
        components={{
          a({ node, ...props }) {
            void node;
            return <a {...props} rel="noopener noreferrer" target="_blank" />;
          },
        }}
        skipHtml
      >
        {safeMarkdown}
      </Markdown>
    </div>
  );
}
