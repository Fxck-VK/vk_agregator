export type LandingToolKind = "chat" | "image" | "video" | "text" | "music" | "catalog";

export type LandingTool = {
  description: string;
  href: string;
  icon: string;
  id: string;
  kind: LandingToolKind;
  name: string;
  priceStarsByQuality?: Readonly<Record<string, number>>;
};

export type LandingNewsItem = {
  description: string;
  href: string;
  id: string;
  imageAlt: string;
  imageSrc: string;
  linkLabel: string;
  title: string;
};

export type LandingModel = {
  description: string;
  href: string;
  icon: string;
  id: string;
  name: string;
  priceStars?: number;
};

export type LandingCapability = {
  description: string;
  href: string;
  id: string;
  imageAlt: string;
  imageSrc: string;
  title: string;
};

export type LandingFaqItem = {
  answer: string;
  id: string;
  question: string;
};

export type LandingFooterLink = {
  href: string;
  label: string;
};

export type LandingFooterGroup = {
  id: string;
  links: LandingFooterLink[];
  title: string;
};
