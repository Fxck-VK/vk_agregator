"use client";

import Image from "next/image";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { ru } from "@/i18n/ru";
import type { ImageModel } from "@/lib/web-api/contracts";

import { loadImageModelCatalog } from "../image-model-catalog-cache";
import { useWorkspaceModelSelection } from "../WorkspaceModelSelection/WorkspaceModelSelection";
import styles from "./WorkspaceModelSelector.module.css";

type CatalogueStatus = "loading" | "ready" | "failure";

function getMinimumPrice(model: ImageModel) {
  const prices = Object.values(model.price_by_quality ?? {});
  return prices.length > 0 ? Math.min(...prices) : null;
}

export function WorkspaceModelSelector() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const requestedModelId = searchParams.get("model");
  const workspaceSelection = useWorkspaceModelSelection();
  const workspaceSelectedModelId = workspaceSelection?.selectedModelId ?? null;
  const setWorkspaceModelId = workspaceSelection?.setSelectedModelId;
  const rootRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [status, setStatus] = useState<CatalogueStatus>("loading");
  const [models, setModels] = useState<ImageModel[]>([]);
  const [selectedModelId, setSelectedModelId] = useState("");
  const [query, setQuery] = useState("");
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    let active = true;

    void loadImageModelCatalog()
      .then((catalogue) => {
        if (!active) {
          return;
        }

        const requestedModelExists = requestedModelId !== null
          && catalogue.items.some((model) => model.id === requestedModelId);
        const workspaceModelExists = workspaceSelectedModelId !== null
          && catalogue.items.some((model) => model.id === workspaceSelectedModelId);
        const initialModelId = requestedModelExists
          ? requestedModelId
          : workspaceModelExists
            ? workspaceSelectedModelId
            : (catalogue.items[0]?.id ?? "");

        setModels(catalogue.items);
        setSelectedModelId(initialModelId);
        if (initialModelId !== "") {
          setWorkspaceModelId?.(initialModelId);
        }
        setStatus(catalogue.items.length > 0 ? "ready" : "failure");
      })
      .catch(() => {
        if (active) {
          setStatus("failure");
        }
      });

    return () => {
      active = false;
    };
  }, [requestedModelId, setWorkspaceModelId, workspaceSelectedModelId]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    searchRef.current?.focus();

    const closeFromOutside = (event: PointerEvent) => {
      if (event.target instanceof Node && !rootRef.current?.contains(event.target)) {
        setIsOpen(false);
        setQuery("");
      }
    };
    const closeFromKeyboard = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setIsOpen(false);
        setQuery("");
        triggerRef.current?.focus();
      }
    };

    document.addEventListener("pointerdown", closeFromOutside);
    document.addEventListener("keydown", closeFromKeyboard);
    return () => {
      document.removeEventListener("pointerdown", closeFromOutside);
      document.removeEventListener("keydown", closeFromKeyboard);
    };
  }, [isOpen]);

  const activeSelectedModelId = workspaceSelectedModelId ?? selectedModelId;
  const selectedModel = models.find((model) => model.id === activeSelectedModelId) ?? null;
  const filteredModels = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase("ru");
    if (normalizedQuery.length === 0) {
      return models;
    }

    return models.filter((model) =>
      `${model.name} ${model.id}`.toLocaleLowerCase("ru").includes(normalizedQuery),
    );
  }, [models, query]);

  const triggerName =
    status === "loading"
      ? ru.modelSelector.loading
      : selectedModel === null
        ? ru.modelSelector.unavailable
        : ru.modelSelector.triggerLabel(selectedModel.name);

  const selectModel = (model: ImageModel) => {
    setSelectedModelId(model.id);
    setWorkspaceModelId?.(model.id);
    setIsOpen(false);
    setQuery("");
    router.push(`/app/image?model=${encodeURIComponent(model.id)}`);
    triggerRef.current?.focus();
  };

  return (
    <div className={styles.root} ref={rootRef}>
      <button
        aria-controls="workspace-model-selector-dialog"
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={triggerName}
        className={styles.trigger}
        disabled={status !== "ready" || selectedModel === null}
        onClick={() => {
          if (isOpen) {
            setQuery("");
          }
          setIsOpen((current) => !current);
        }}
        ref={triggerRef}
        type="button"
      >
        <span aria-hidden="true" className={styles.modelIcon}>✦</span>
        <span className={styles.triggerText}>
          {status === "loading" ? ru.modelSelector.loadingShort : (selectedModel?.name ?? ru.modelSelector.unavailable)}
        </span>
        <span aria-hidden="true" className={`${styles.chevron} ${isOpen ? styles.chevronOpen : ""}`}>
          <Image alt="" height={18} src={assetPaths.icons.ui.chevronDown} unoptimized width={18} />
        </span>
      </button>

      {isOpen ? (
        <section
          aria-label={ru.modelSelector.dialogLabel}
          className={styles.popover}
          id="workspace-model-selector-dialog"
          role="dialog"
        >
          <div className={styles.searchRow}>
            <span aria-hidden="true" className={styles.searchIcon}>⌕</span>
            <input
              aria-label={ru.modelSelector.searchLabel}
              className={styles.search}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={ru.modelSelector.searchPlaceholder}
              ref={searchRef}
              type="search"
              value={query}
            />
          </div>

          <div className={styles.scrollArea}>
            <h2 className={styles.category}>{ru.modelSelector.imagesCategory}</h2>
            {filteredModels.length > 0 ? (
              <ul aria-label={ru.modelSelector.imagesCategory} className={styles.options}>
                {filteredModels.map((model) => {
                  const minimumPrice = getMinimumPrice(model);
                  const isSelected = model.id === activeSelectedModelId;

                  return (
                    <li key={model.id}>
                      <button
                        aria-pressed={isSelected}
                        className={`${styles.option} ${isSelected ? styles.optionSelected : ""}`}
                        onClick={() => selectModel(model)}
                        type="button"
                      >
                        <span aria-hidden="true" className={styles.optionIcon}>✦</span>
                        <span className={styles.optionCopy}>
                          <span className={styles.optionTitle}>{model.name}</span>
                          <span className={styles.optionDescription}>
                            {model.supports_reference_image
                              ? ru.modelSelector.referenceDescription
                              : ru.modelSelector.textDescription}
                          </span>
                        </span>
                        {minimumPrice !== null ? <span className={styles.price}>✦ {minimumPrice}</span> : null}
                        <span aria-hidden="true" className={styles.selectionMark}>{isSelected ? "●" : ""}</span>
                      </button>
                    </li>
                  );
                })}
              </ul>
            ) : (
              <p className={styles.empty} role="status">{ru.modelSelector.empty}</p>
            )}
          </div>

          <Link
            className={styles.catalogueLink}
            href="/app/models"
            onClick={() => {
              setIsOpen(false);
              setQuery("");
            }}
            prefetch={false}
          >
            <span>{ru.modelSelector.openCatalogue}</span>
            <span aria-hidden="true">→</span>
          </Link>
        </section>
      ) : null}
    </div>
  );
}
