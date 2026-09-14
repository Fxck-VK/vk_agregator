export type ModelPresentation = {
  artworkSrc?: string;
  description: string;
  href: string;
};

type ModelPresentationSource = {
  artworkSrc?: string;
  description?: string;
  id: string;
  name: string;
};

export function getModelPresentation(model: ModelPresentationSource): ModelPresentation {
  return {
    artworkSrc: model.artworkSrc,
    description: model.description ?? `${model.name} доступна для выбора в NeiroHub.`,
    href: `/app/chats?model=${encodeURIComponent(model.id)}`,
  };
}
