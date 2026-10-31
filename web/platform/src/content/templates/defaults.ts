// Named, immutable data sources; no component or server runtime imports.
import {homeEn} from "@/i18n/public/home-en";
import {aiToolsEn} from "@/i18n/public/ai-tools-en";
import {aiToolsReferenceEn} from "@/i18n/public/ai-tools-reference-en";
import {imageGeneratorEn} from "@/i18n/public/image-generator-en";
import {imageGeneratorReferenceEn} from "@/i18n/public/image-generator-reference-en";
import {klingEn} from "@/i18n/public/kling-en";
import {klingReferenceEn} from "@/i18n/public/kling-reference-en";
import {klingCatalogEn} from "@/i18n/public/kling-catalog-en";
import {kling30En} from "@/i18n/public/kling30-en";
import {kling30ReferenceEn} from "@/i18n/public/kling30-reference-en";
import {tattooEn} from "@/i18n/public/tattoo-en";
import {tattooReferenceEn} from "@/i18n/public/tattoo-reference-en";
import {marketingEn} from "@/i18n/public/marketing-en";
import {marketingReferenceEn} from "@/i18n/public/marketing-reference-en";
import {comparisonEn} from "@/i18n/public/comparison-en";
import {authorPath} from "@/i18n/public/author-en";
import {comparisonReferenceEn} from "@/i18n/public/comparison-reference-en";
import {avatarModelsEn} from "@/i18n/public/avatar-models-en";
import {artPromptsEn} from "@/i18n/public/art-prompts-en";
import {artPromptsReferenceEn} from "@/i18n/public/art-prompts-reference-en";
import {courseEn} from "@/i18n/public/course-en";
import {documentationEn} from "@/i18n/public/documentation-en";
import {documentationArticles} from "@/i18n/public/documentation-en";
import {documentationGroups} from "@/i18n/public/documentation-en";
import {workflowEn} from "@/i18n/public/workflow-en";
import {workflowReferenceEn} from "@/i18n/public/workflow-reference-en";
import {productCampaignWorkflow} from "@/i18n/public/workflow-en";
import {collectionEn} from "@/i18n/public/collection-en";
import {cinematicCollection} from "@/i18n/public/collection-en";
import {publicExampleEn} from "@/i18n/public/public-example-en";
import {flowerExample} from "@/i18n/public/public-example-en";
import {materialCategoryEn} from "@/i18n/public/material-category-en";
import {natureCategory} from "@/i18n/public/material-category-en";
import {natureMaterials} from "@/i18n/public/material-category-en";
import {gypsophilaMaterial} from "@/i18n/public/material-detail-en";
import {materialFilters} from "@/i18n/public/material-category-en";
import {materialDetailEn} from "@/i18n/public/material-detail-en";
import {mcpEn} from "@/i18n/public/integration-en";
import {mcpPrompts} from "@/i18n/public/integration-en";
import {mcpClients} from "@/i18n/public/integration-en";
import {integrationReference} from "@/i18n/public/integration-reference-en";
import {mcpIntegration} from "@/i18n/public/integration-en";
import {pricingEn} from "@/i18n/public/pricing-en";
import {pricingReference} from "@/i18n/public/pricing-reference-en";
import {businessEn} from "@/i18n/public/business-en";
import {businessReference} from "@/i18n/public/business-reference-en";
import {businessPlan} from "@/i18n/public/business-en";
import {customerStoryEn} from "@/i18n/public/customer-story-en";
import {footballCaseStudy} from "@/i18n/public/customer-story-en";
import {customerStoryReference} from "@/i18n/public/customer-story-reference-en";
import {aboutEn} from "@/i18n/public/about-en";
import {aboutReference} from "@/i18n/public/about-reference-en";
import {aboutModels} from "@/i18n/public/about-en";
import {aboutStats} from "@/i18n/public/about-en";
import {eventProjectEn} from "@/i18n/public/event-project-en";
import {eventProjectReference} from "@/i18n/public/event-project-reference-en";
import {eventProject} from "@/i18n/public/event-project-en";
import {authorEn} from "@/i18n/public/author-en";
import {authorProfile} from "@/i18n/public/author-en";
import {authorMaterials} from "@/features/author/author-catalog";
import {initialMaterialFilters} from "@/features/material-category/category-model";
export const templateDefaults = {
  T01: {homeEn},
  T02: {aiToolsEn,aiToolsReferenceEn},
  T03: {imageGeneratorEn,imageGeneratorReferenceEn},
  T04: {klingEn,klingReferenceEn,klingCatalogEn},
  T05: {kling30En,klingEn,kling30ReferenceEn},
  T06: {tattooEn,tattooReferenceEn},
  T07: {marketingEn,marketingReferenceEn},
  T08: {comparisonEn,authorPath,comparisonReferenceEn,avatarModelsEn},
  T09: {artPromptsEn,authorPath,artPromptsReferenceEn,comparisonReferenceEn},
  T10: {courseEn},
  T11: {documentationEn,documentationArticles,documentationGroups},
  T12: {workflowEn,workflowReferenceEn,productCampaignWorkflow},
  T13: {collectionEn,cinematicCollection},
  T14: {publicExampleEn,flowerExample},
  T15: {materialCategoryEn,natureCategory,natureMaterials,gypsophilaMaterial,materialFilters,initialMaterialFilters},
  T16: {materialDetailEn,publicExampleEn,gypsophilaMaterial},
  T17: {mcpEn,mcpPrompts,mcpClients,integrationReference,mcpIntegration},
  T18: {pricingEn,pricingReference},
  T19: {businessEn,businessReference,businessPlan},
  T20: {customerStoryEn,footballCaseStudy,customerStoryReference},
  T21: {aboutEn,aboutReference,aboutModels,aboutStats},
  T22: {eventProjectEn,eventProjectReference,eventProject},
  T23: {},
  T24: {authorEn,authorProfile,authorPath,authorMaterials},
};
