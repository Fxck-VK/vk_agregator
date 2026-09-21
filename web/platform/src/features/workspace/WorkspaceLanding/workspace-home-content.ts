import { getTranslator, type Translator } from "@/i18n/messages";

import { assetPaths } from "@/assets/asset-paths";

export type WorkspaceHomeFaq = {
  answer: string;
  question: string;
};

export function getCapabilityLinks(msg: Translator = getTranslator("ru")) {

  return [
    { href: "/app/chats", label: msg("workspaceHomeContent.answersToQuestions"), icon: assetPaths.icons.features.answers },
    { href: "/app/image", label: msg("workspaceHomeContent.imageGeneration"), icon: assetPaths.icons.features.generateImage },
    { href: "/app/image", label: msg("workspaceHomeContent.workingWithReferences"), icon: assetPaths.icons.features.references },
    { href: "/app/files", label: msg("workspaceHomeContent.fileLibrary"), icon: assetPaths.icons.features.fileLibrary },
    { href: "/app/models", label: msg("workspaceHomeContent.chooseAnAiModel"), icon: assetPaths.icons.features.selectAi },
    { href: "/app/inspiration", label: msg("workspaceHomeContent.promptIdeas"), icon: assetPaths.icons.features.promptIdeas },
  ] as const;
}

export const capabilityLinks = getCapabilityLinks();

export function getFrequentlyAskedQuestions(msg: Translator = getTranslator("ru")): WorkspaceHomeFaq[] {

  return [
    {
      question: msg("workspaceHomeContent.whatIsNeirohub"),
      answer:
        msg("workspaceHomeContent.neirohubIsOneWorkspaceForChattingWith"),
    },
    {
      question: msg("workspaceHomeContent.whatAreNeirohubSOwnAiModels"),
      answer:
        msg("workspaceHomeContent.thisIsWhatWeCallModelsAvailable"),
    },
    {
      question: msg("workspaceHomeContent.whatAreTokensAndSubscriptions"),
      answer:
        msg("workspaceHomeContent.inNeirohubStarsAreBalanceUnitsUsed"),
    },
    {
      question: msg("workspaceHomeContent.howDoIBuyASubscription"),
      answer:
        msg("workspaceHomeContent.subscriptionsAreNotCurrentlySoldToRun"),
    },
    {
      question: msg("workspaceHomeContent.isThereFreeAccess"),
      answer:
        msg("workspaceHomeContent.youCanOpenTheWorkspaceAndExplore"),
    },
  ];
}

export const frequentlyAskedQuestions = getFrequentlyAskedQuestions();
