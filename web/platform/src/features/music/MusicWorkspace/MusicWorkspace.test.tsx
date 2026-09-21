import { useState } from "react";

import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  MusicWorkspace,
  type MusicCreationMode,
  type MusicOperationID,
  type MusicTrack,
  type MusicWorkspaceDraft,
  type MusicWorkspaceModel,
  type MusicWorkspaceOperation,
  type MusicWorkspaceProps,
} from "./MusicWorkspace";

const readyModels: readonly MusicWorkspaceModel[] = [
  {
    availability: "available",
    description: "Основная музыкальная модель",
    enabled: true,
    id: "suno_v6",
    name: "Suno V6",
  },
  {
    availability: "unverified",
    enabled: false,
    id: "suno_v6_wild",
    name: "Suno V6 Wild",
    operationDetails: ["Ожидает live verification"],
    statusReason: "Модель ждёт отдельной проверки",
  },
];

const baseDraft: MusicWorkspaceDraft = {
  actionParameters: {},
  descriptionPrompt: "",
  instrumental: false,
  lyrics: "",
  maxMode: false,
  outputFormat: "mp3",
  promptWeight: 0.6,
  style: "",
  styleWeight: 0.5,
  suggestedLyrics: "Первый куплет\nПрипев",
  targetDurationMode: "auto",
  targetDurationSec: 120,
  title: "",
  variety: 0.4,
};

const generateOperation: MusicWorkspaceOperation = {
  enabled: true,
  estimateCredits: 25,
  id: "generate",
  supportsAudioFormat: true,
  supportsMaxMode: true,
};

const mashupOperation: MusicWorkspaceOperation = {
  details: ["Исходные треки: 2", "Результат: аудио"],
  enabled: true,
  estimateCredits: 30,
  id: "mashup",
  requirement: { maxTracks: 2, minTracks: 2 },
};

const tracks: readonly MusicTrack[] = [
  {
    artifactPlaybackUrl: "/web/v1/music-artifacts/11111111-1111-4111-8111-111111111111",
    durationSec: 121,
    id: "track-a",
    lyrics: "lyrics a",
    originalIndex: 0,
    sourceJobId: "job-a",
    title: "First",
  },
  {
    artifactPlaybackUrl: "/web/v1/music-artifacts/22222222-2222-4222-8222-222222222222",
    durationSec: 98,
    id: "track-b",
    originalIndex: 1,
    sourceJobId: "job-b",
    title: "Second",
  },
  {
    artifactPlaybackUrl: "https://cdn.example.invalid/private.mp3",
    durationSec: 64,
    id: "track-c",
    originalIndex: 5,
    sourceJobId: "job-c",
    title: "Remote",
  },
];

type HarnessProps = Partial<MusicWorkspaceProps> & {
  initialDraft?: MusicWorkspaceDraft;
  initialMode?: MusicCreationMode;
  initialSelectedTrackIds?: readonly string[];
};

function StatefulWorkspace({
  initialDraft = baseDraft,
  initialMode = "description",
  initialSelectedTrackIds = [],
  ...props
}: HarnessProps) {
  const [draft, setDraft] = useState(initialDraft);
  const [mode, setMode] = useState<MusicCreationMode>(initialMode);
  const [selectedTrackIds, setSelectedTrackIds] = useState<readonly string[]>(initialSelectedTrackIds);
  const [activeOperationId, setActiveOperationId] = useState<MusicOperationID | null>(
    props.activeOperationId ?? null,
  );

  return (
    <MusicWorkspace
      activeOperationId={activeOperationId}
      draft={draft}
      mode={mode}
      models={readyModels}
      onActiveOperationChange={setActiveOperationId}
      onCancelConfirmation={vi.fn()}
      onConfirmAction={vi.fn()}
      onDraftChange={setDraft}
      onModeChange={setMode}
      onModelChange={vi.fn()}
      onRequestConfirmation={vi.fn()}
      onSelectedTrackIdsChange={setSelectedTrackIds}
      operations={[generateOperation]}
      selectedModelId="suno_v6"
      selectedTrackIds={selectedTrackIds}
      tracks={[]}
      {...props}
    />
  );
}

describe("MusicWorkspace", () => {
  it("shows Lyria controls without cached Suno options or duration above 240 seconds", () => {
    render(<StatefulWorkspace
      selectedModelId="lyria_3_5"
      models={[{ id: "lyria_3_5", name: "Lyria 3.5", enabled: true, availability: "available" }]}
      operations={[{ id: "generate", enabled: true, estimateCredits: 40 }]}
      initialDraft={{ ...baseDraft, descriptionPrompt: "synthetic piano", maxMode: true, targetDurationMode: "custom", targetDurationSec: 360 }}
    />);
    expect(screen.queryByRole("checkbox", { name: "Max mode" })).not.toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: "Инструментал" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Формат:/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("slider", { name: "Вес описания" })).not.toBeInTheDocument();
    expect(screen.getByRole("slider", { name: "Целевая длительность" })).toHaveAttribute("max", "240");
    expect(screen.getByRole("slider", { name: "Целевая длительность" })).toHaveValue("240");
    expect(screen.getByRole("button", { name: "Сгенерировать" })).toBeEnabled();
  });

  it("shows mode-specific fields and preserves controlled drafts across modes", () => {
    const onRequestOwnedUpload = vi.fn();
    render(<StatefulWorkspace onRequestOwnedUpload={onRequestOwnedUpload} uploadsEnabled />);

    fireEvent.change(screen.getByRole("textbox", { name: "Описание трека" }), {
      target: { value: "dark synth pop with a dry vocal" },
    });
    fireEvent.click(screen.getByRole("tab", { name: "Свой текст" }));
    fireEvent.change(screen.getByRole("textbox", { name: "Свой текст песни" }), {
      target: { value: "verse one\nchorus" },
    });
    fireEvent.click(screen.getByRole("tab", { name: "Загрузка" }));
    fireEvent.click(screen.getByRole("button", { name: "Выбрать аудио" }));

    expect(onRequestOwnedUpload).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("tab", { name: "Описание" }));
    expect(screen.getByRole("textbox", { name: "Описание трека" })).toHaveValue(
      "dark synth pop with a dry vocal",
    );

    fireEvent.click(screen.getByRole("tab", { name: "Свой текст" }));
    expect(screen.getByRole("textbox", { name: "Свой текст песни" })).toHaveValue("verse one\nchorus");
  });

  it("requests server preparation without guessing the confirmation price", () => {
    const onRequestConfirmation = vi.fn();
    render(
      <StatefulWorkspace
        initialDraft={{ ...baseDraft, descriptionPrompt: "make a chorus" }}
        onRequestConfirmation={onRequestConfirmation}
        operations={[{ ...generateOperation, estimateCredits: 25, maxEstimateCredits: 40, quote: null }]}
      />,
    );

    expect(screen.getByLabelText("Оценка до: 40 звёзд")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Сгенерировать" }));

    expect(onRequestConfirmation).toHaveBeenCalledTimes(1);
    expect(onRequestConfirmation.mock.calls[0]).toHaveLength(1);
    expect(onRequestConfirmation.mock.calls[0][0]).toMatchObject({
      modelId: "suno_v6",
      operationId: "generate",
    });
  });

  it("blocks custom-only generation controls in description mode unless lyrics or instrumental mode is used", () => {
    const onRequestConfirmation = vi.fn();
    render(
      <StatefulWorkspace
        initialDraft={{ ...baseDraft, descriptionPrompt: "anthem idea", maxMode: true }}
        onRequestConfirmation={onRequestConfirmation}
        operations={[{ ...generateOperation, supportsMaxMode: true }]}
      />,
    );

    expect(screen.getByText("Max mode, persona и пользовательская модель требуют свой текст или инструментал")).toBeVisible();
    expect(screen.getByRole("button", { name: "Сгенерировать" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Сгенерировать" }));
    expect(onRequestConfirmation).not.toHaveBeenCalled();
  });

  it("presents unavailable catalog states without enabling model actions", () => {
    const onRequestConfirmation = vi.fn();
    render(
      <StatefulWorkspace
        initialDraft={{ ...baseDraft, descriptionPrompt: "make a chorus" }}
        models={[]}
        onRequestConfirmation={onRequestConfirmation}
        state={{ modelsMessage: "Аудиомодели ждут verification", modelsStatus: "empty" }}
      />,
    );

    expect(screen.getByRole("status")).toHaveTextContent("Аудиомодели ждут verification");
    expect(screen.getByRole("button", { name: "Сгенерировать" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Сгенерировать" }));
    expect(onRequestConfirmation).not.toHaveBeenCalled();
  });

  it("renders every provided track and only plays same-origin artifact URLs", () => {
    const { container } = render(<StatefulWorkspace tracks={tracks} />);

    expect(screen.getByText("Трек 1")).toBeVisible();
    expect(screen.getByText("Трек 2")).toBeVisible();
    expect(screen.getByText("Трек 6")).toBeVisible();
    expect(screen.getByText("2:01")).toBeVisible();
    expect(screen.getByText("1:38")).toBeVisible();
    expect(screen.getByText("1:04")).toBeVisible();
    expect(screen.queryByText(/job-a/)).not.toBeInTheDocument();
    expect(container.querySelectorAll("audio")).toHaveLength(2);
    expect(screen.getByText("Источник недоступен для воспроизведения")).toBeVisible();
  });

  it("shows text tool results and safe artifact downloads", () => {
    render(
      <StatefulWorkspace
        results={[
          {
            artifacts: [
              {
                format: "txt",
                kind: "lyrics",
                label: "Скачать файл TXT",
                url: "/web/v1/music-artifacts/33333333-3333-4333-8333-333333333333",
              },
            ],
            bpm: { average: 126, maximum: 130, minimum: 122 },
            id: "result-a",
            lyrics: [{ tags: "verse, chorus", text: "Первый куплет\nПрипев", title: "Текст песни" }],
            tags: "dream pop, bright drums",
          },
        ]}
      />,
    );

    expect(screen.getByText("Результаты инструментов")).toBeVisible();
    expect(screen.getByText("Теги: dream pop, bright drums")).toBeVisible();
    expect(screen.getByText("BPM: 126 · диапазон 122–130")).toBeVisible();
    expect(screen.getAllByText((_, element) => element?.textContent === "Первый куплет\nПрипев")).toHaveLength(2);
    expect(screen.getByRole("link", { name: "Скачать файл TXT" })).toHaveAttribute(
      "href",
      "/web/v1/music-artifacts/33333333-3333-4333-8333-333333333333",
    );
  });

  it("requires a second source track for mashup and sends stable source IDs", () => {
    const onRequestConfirmation = vi.fn();
    render(
      <StatefulWorkspace
        onRequestConfirmation={onRequestConfirmation}
        operations={[generateOperation, mashupOperation]}
        tracks={tracks}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /^Mashup/ }));
    expect(screen.getByText("Нужен второй трек")).toBeVisible();
    expect(screen.getByRole("button", { name: "Подготовить Mashup" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Выбрать трек 1" }));
    expect(screen.getByRole("button", { name: "Подготовить Mashup" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Выбрать трек 2" }));
    const actionForm = screen.getByRole("button", { name: "Подготовить Mashup" }).closest("div");
    expect(actionForm).not.toBeNull();
    expect(within(actionForm!).getByLabelText("Второй трек для mashup")).toHaveValue("track-b");
    fireEvent.change(within(actionForm!).getByRole("textbox", { name: "Промпт операции" }), {
      target: { value: "blend these hooks" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Подготовить Mashup" }));

    expect(onRequestConfirmation).toHaveBeenCalledTimes(1);
    const [request] = onRequestConfirmation.mock.calls[0];
    expect(request.operationId).toBe("mashup");
    expect(request.sourceTrackIds).toEqual(["track-a", "track-b"]);
    expect(request.sourceJobIds).toEqual(["job-a", "job-b"]);
  });

  it("ignores retained tracks and uploads for zero-input helper operations", () => {
    const onRequestConfirmation = vi.fn();
    render(
      <StatefulWorkspace
        activeOperationId="lyrics"
        initialDraft={{
          ...baseDraft,
          actionParameters: { lyrics: { prompt: "write a sunrise chorus" } },
          ownedUploads: [{
            durationSec: 42,
            id: "33333333-3333-4333-8333-333333333333",
            label: "stale.wav",
          }],
        }}
        initialMode="upload"
        initialSelectedTrackIds={["track-a"]}
        onRequestConfirmation={onRequestConfirmation}
        operations={[
          generateOperation,
          {
            enabled: true,
            id: "lyrics",
            requirement: { maxTracks: 0, maxUploads: 0, minTracks: 0, minUploads: 0 },
          },
        ]}
        tracks={tracks}
      />,
    );

    expect(screen.queryByText(/Источник:/)).not.toBeInTheDocument();
    const runButton = screen.getByRole("button", { name: "Подготовить Текст песни" });
    expect(runButton).toBeEnabled();

    fireEvent.click(runButton);

    expect(onRequestConfirmation).toHaveBeenCalledTimes(1);
    const [request] = onRequestConfirmation.mock.calls[0];
    expect(request.operationId).toBe("lyrics");
    expect(request.sourceTrackIds).toEqual([]);
    expect(request.sourceJobIds).toEqual([]);
  });
});
