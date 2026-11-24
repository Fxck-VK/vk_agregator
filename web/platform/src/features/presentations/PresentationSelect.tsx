"use client";

import { useRef, useState } from "react";
import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import { PopoverOption } from "@/components/ui/PopoverOption/PopoverOption";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import styles from "./PresentationsPage.module.css";

export function PresentationSelect<Value extends string>({ label, value, options, onChange, portalLayer }: {
  label: string;
  value: Value;
  options: readonly { value: Value; label: string }[];
  onChange: (value: Value) => void;
  portalLayer?: number;
}) {
  const trigger = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const selected = options.find(option => option.value === value)?.label;

  return <div className={styles.select}>
    <span className={styles.selectLabel}>{label}</span>
    <InputControlChip ref={trigger} className={styles.selectTrigger} aria-label={`${label}: ${selected}`} aria-expanded={open} aria-haspopup="dialog" onClick={() => setOpen(current => !current)}>
      <span>{selected}</span><svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8"><path d="m6 9 6 6 6-6" /></svg>
    </InputControlChip>
    <PopoverPanel label={label} anchorRef={trigger} isOpen={open} onClose={() => setOpen(false)} width={320} portalLayer={portalLayer}>
      <div role="radiogroup" aria-label={label} className={styles.selectOptions}>
        {options.map(option => <PopoverOption key={option.value} selected={value === option.value} onClick={() => {
          onChange(option.value);
          setOpen(false);
          trigger.current?.focus();
        }}>{option.label}</PopoverOption>)}
      </div>
    </PopoverPanel>
  </div>;
}
