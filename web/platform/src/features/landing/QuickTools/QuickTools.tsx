"use client";

import Link from "next/link";

import { useLandingToolSelection } from "../LandingToolSelection/LandingToolSelection";
import { landingTools } from "../landing-content";
import styles from "./QuickTools.module.css";

export function QuickTools() {
  const { selectedTool, selectTool } = useLandingToolSelection();

  return (
    <ul aria-label="Быстрый выбор инструмента" className={styles.row}>
      {landingTools.map((tool) => {
        const content = (
          <>
            <span aria-hidden="true" className={styles.icon}>{tool.icon}</span>
            <span className={styles.copy}><strong>{tool.name}</strong><small>{tool.description}</small></span>
          </>
        );

        if (tool.kind === "catalog") {
          return (
            <li key={tool.id}>
              <Link className={styles.card} data-testid="quick-tool" href={tool.href}>
                {content}
              </Link>
            </li>
          );
        }

        return (
          <li key={tool.id}>
            <button
              aria-pressed={selectedTool.id === tool.id}
              className={styles.card}
              data-active={selectedTool.id === tool.id}
              data-testid="quick-tool"
              onClick={() => selectTool(tool.id)}
              type="button"
            >
              {content}
            </button>
          </li>
        );
      })}
    </ul>
  );
}
