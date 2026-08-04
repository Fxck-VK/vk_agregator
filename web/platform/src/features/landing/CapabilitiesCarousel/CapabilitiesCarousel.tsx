"use client";

import Image from "next/image";
import Link from "next/link";
import { useRef } from "react";

import { landingCapabilities } from "../landing-content";
import styles from "./CapabilitiesCarousel.module.css";

export function CapabilitiesCarousel() {
  const listRef = useRef<HTMLDivElement>(null);
  const move = (direction: number) => {
    const list = listRef.current;
    if (list && typeof list.scrollBy === "function") list.scrollBy({ behavior: "smooth", left: direction * Math.min(list.clientWidth * .82, 720) });
  };

  return (
    <div>
      <div className={styles.heading}>
        <div><p>Всё в одном месте</p><h2 id="capabilities-title">Возможности платформы</h2></div>
        <div><button aria-label="Предыдущая возможность" onClick={() => move(-1)} type="button">←</button><button aria-label="Следующая возможность" onClick={() => move(1)} type="button">→</button></div>
      </div>
      <div className={styles.track} ref={listRef}>
        {landingCapabilities.map((capability) => (
          <Link className={styles.card} data-testid="capability-card" href={capability.href} key={capability.id}>
            <Image alt={capability.imageAlt} fill loading="lazy" sizes="(max-width: 680px) 86vw, 34vw" src={capability.imageSrc} />
            <span><strong>{capability.title}</strong><small>{capability.description}</small></span>
          </Link>
        ))}
      </div>
    </div>
  );
}
