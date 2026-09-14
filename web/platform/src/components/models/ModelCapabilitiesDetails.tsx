import type { CapabilityProfile, InputCapability, ModelCapabilities } from "./model-capabilities";
import styles from "./ModelCapabilitiesDetails.module.css";

const statusLabels: Record<InputCapability["support"], string> = { supported: "Да", unsupported: "Нет", unknown: "Не подтверждено" };
function inputLabel(input: InputCapability) {
  if (input.support !== "supported") return statusLabels[input.support];
  const count = input.max_count === null ? "максимум не подтверждён" : `до ${input.max_count}`;
  const files = input.extensions?.length ? `; ${input.extensions.join(", ")}` : "; форматы не подтверждены";
  return `Да, ${count}${files}`;
}
function values(items: (string | number)[] | null | undefined) { return items?.length ? items.join(", ") : "Не задано / не подтверждено"; }
const frameLabels: Record<string, string> = { required: "Обязательно", optional: "Необязательно", unsupported: "Не поддерживается", unknown: "Не подтверждено" };
const audioLabels: Record<string, string> = { optional: "Со звуком или без", generated: "Со звуком", silent: "Без звука", preserve_source: "Можно сохранить звук исходного видео", unknown: "Не подтверждено" };

export function capabilityRows(p: CapabilityProfile): [string, string][] {
  if (p.text) return [["Фото", inputLabel(p.text.images)], ["Видео", inputLabel(p.text.videos)], ["Другие файлы (TXT, DOC, PDF и др.)", inputLabel(p.text.files)]];
  if (p.image) return [
    ["Фото на вход", inputLabel(p.image.images)], ["Пропорции", values(p.image.aspect_ratios)],
    ["Разрешение", values(p.image.resolutions)], ["Детализация", values(p.image.quality_modes)], ["Скорость", values(p.image.speed_modes)],
    ["Результатов за запрос", p.image.max_output_count?.toString() ?? "Не подтверждено"],
    ...(p.image.max_combined_images === null ? [] : [["Всего фото на входе и выходе", String(p.image.max_combined_images)] as [string, string]]),
  ];
  if (p.video) {
    const v = p.video;
    const d = v.duration;
    const range = d.min_seconds !== null && d.max_seconds !== null ? `${d.min_seconds}–${d.max_seconds} с` : "Границы не подтверждены";
    const duration = d.mode === "automatic" ? `Автоматически. ${range}` : d.mode === "reference_video" ? `По исходному видео. ${range}` : range;
    return [["Фото на вход", inputLabel(v.images)], ["Видео на вход", inputLabel(v.videos)],
      ["Длительность", duration], ...(d.allowed_seconds?.length ? [["Доступные длительности, с", values(d.allowed_seconds)] as [string, string]] : []),
      ...Object.entries(d.by_resolution ?? {}).map(([resolution, seconds]): [string, string] => [`Длительность для ${resolution}, с`, values(seconds)]),
      ...Object.entries(d.max_by_orientation ?? {}).map(([orientation, seconds]): [string, string] => [`Максимум при ориентации ${orientation}`, `${seconds} с`]),
      ["Разрешение", values(v.resolutions)], ["Режим качества", values(v.quality_modes)], ["Пропорции", values(v.aspect_ratios)],
      ["Звук", audioLabels[v.audio.mode] ?? "Не подтверждено"],
      ["Начальный кадр", frameLabels[v.start_frame] ?? "Не подтверждено"], ["Конечный кадр", frameLabels[v.end_frame] ?? "Не подтверждено"],
      ...(v.allowed_image_counts?.length ? [["Допустимое число фото", values(v.allowed_image_counts)] as [string, string]] : [])];
  }
  if (p.audio) return [["Аудио на вход", inputLabel(p.audio.audio)], ["Видео на вход", inputLabel(p.audio.videos)], ["Результат", p.audio.output]];
  return [];
}

export function ModelCapabilitiesDetails({ capabilities }: { capabilities?: ModelCapabilities }) {
  if (!capabilities) return null;
  return <details className={styles.details}>
    <summary>Возможности модели</summary>
    {([["В этом интерфейсе", capabilities.application], ["В API провайдера", capabilities.api]] as const).map(([title, profile]) =>
      <section key={title} aria-label={title}>
        <p><strong>{title}</strong></p>
        <dl>{capabilityRows(profile).map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}</dl>
        {profile.notes?.map((note) => <p key={note}><small>{note}</small></p>)}
      </section>)}
  </details>;
}
