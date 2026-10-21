import { BrandText } from "@/components/brand/BrandName/BrandName";
import Image from "next/image";

import { assetPaths } from "@/assets/asset-paths";

import styles from "./FAQ.module.css";

export type FAQItem = {
  answer: string;
  question: string;
};

export type FAQProps = {
  items: readonly FAQItem[];
  /** A document-unique group name limits this list to one open answer. */
  name?: string;
};

export function FAQ({ items, name }: Readonly<FAQProps>) {
  return (
    <div className={styles.list} data-ui="faq">
      {items.map((item) => (
        <details key={item.question} name={name}>
          <summary>
            <span><BrandText>{item.question}</BrandText></span>
            <Image
              alt=""
              className={styles.arrow}
              height={10}
              src={assetPaths.icons.ui.faqArrow}
              width={18}
            />
          </summary>
          <p><BrandText>{item.answer}</BrandText></p>
        </details>
      ))}
    </div>
  );
}
