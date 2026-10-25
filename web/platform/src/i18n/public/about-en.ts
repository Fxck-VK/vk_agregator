import {brandName} from '@/config/brand';
import {homeEn} from './home-en';
import {aboutReference as media} from './about-reference-en';
export type CompanyProfile={readonly name:string;readonly supportEmail:string;readonly foundedOn:null;readonly officeAddress:null;readonly people:readonly {name:string;role:string}[]};
export const companyProfile:CompanyProfile={name:brandName,supportEmail:'support@epifera.com',foundedOn:null,officeAddress:null,people:[]};
export const aboutCreativeAreas=[{name:'Images',href:'/app/image'},{name:'Video',href:'/app/models'},{name:'Music',href:'/app/music'},{name:'Speech',href:'/app/speech'},{name:'Chat',href:'/app'}] as const;
export const aboutStats=[{value:String(aboutCreativeAreas.length),label:'Creative areas to explore'},{value:'1',label:'Workspace for your ideas'},{value:'2',label:'Calendar months for generated files'},{value:'1',label:'Support address for your questions'}] as const;
export type AboutModel={readonly id:string;readonly name:string;readonly poster:string;readonly video?:string};
const clips:Record<string,string>={nano_banana_pro:media.NanoBanana.src,video_kling_v3:media.Kling.src,video_veo_3_1_fast:media.Veo3Flow.src,seedream_5_0_lite:media.Seedream.src,flux_2_pro:media.Flux2.src};
export const aboutModels:readonly AboutModel[]=[...homeEn.models.items.map(m=>({id:m.id,name:m.name,poster:'/assets/images/marketing/'+m.image,video:clips[m.id]})),{id:'grok_imagine_1_5_video',name:'Grok Imagine',poster:media['scenario-1'].src,video:media.Grok.src},{id:'happyhorse_1_0',name:'Happy Horse',poster:media['scenario-2'].src,video:media.Happyhorse.src},{id:'seedance_2_0',name:'Seedance',poster:media['scenario-3'].src,video:media.Seedance.src},{id:'video_runway_gen4_5',name:'Runway',poster:media['scenario-4'].src,video:media.Runway.src},{id:'wan_3_0',name:'Wan',poster:media['scenario-5'].src,video:media.Wan.src},{id:'video_minimax_h3',name:'MiniMax',poster:media['scenario-6'].src,video:media.MiniMax.src}];
export const aboutEn={
 metadata:{title:`About ${brandName} — Creative tools and contacts`,description:`Get to know ${brandName}: a creative workspace for images, video, music, speech and chat. Explore the tools and contact support.`},
 skip:'Skip to company information',announcement:'A workspace for your next idea',title:`${brandName} brings your creative tools together`,description:'Images, video, music, speech and chat. Explore the possibilities, shape the direction, and make the next decision yours.',discover:`Discover ${brandName}`,aboutLink:`About ${brandName}`,
 stats:{title:'A clear starting point for your creativity',description:'Useful ways to work, with the tools and information close at hand.'},
 people:{title:`The work behind ${brandName}`,description:'From choosing a model to reviewing a result, these are the parts of the workspace we focus on.',action:'Get in touch with the project',items:[
 {title:'Creative tools',description:'Find a starting point',href:'/ai'},{title:'Model choices',description:'Explore the catalog',href:'/app/models'},{title:'Interface',description:'Your creative workspace',href:'/app'},
 {title:'Images',description:'Shape the next frame',href:'/ai/image-generator'},{title:'Motion',description:'Explore video models',href:'/ai/kling'},{title:'Music',description:'Develop a sound direction',href:'/app/music'},
 {title:'Speech',description:'Give your ideas a voice',href:'/app/speech'},{title:'Conversations',description:'Continue your thinking',href:'/app/chats'},{title:'Created files',description:'Review your outputs',href:'/app/files'},
 {title:'Workflow guides',description:'Work through the stages',href:'/workflows/product-campaign'},{title:'Learning',description:'Build useful habits',href:'/academy/courses/epifera-essentials'},{title:'Inspiration',description:'Explore visual directions',href:'/inspiration/cinematic-scenes'},
 {title:'Account access',description:'Manage your account',href:'/ai/docs/account-management'},{title:'Payment terms',description:'Understand the offer',href:'/legal/paid-access'},{title:'Data and privacy',description:'Read the actual policy',href:'/legal/privacy-policy'},
 {title:'Team planning',description:'Discuss requirements',href:'/business'},{title:'Connections',description:'Explore the planned concept',href:'/mcp'},{title:'Support',description:'Find an answer',href:'/ai/docs'},
 ]},
 scenarios:{title:'Many kinds of creative work, one starting point',description:'Editorial ways to explore the workspace. These examples are ideas, not customer testimonials.',items:[
 {title:'Find a visual direction',description:'Start with a mood, a brief and a few references. Explore what the first image could become.',href:'/ai/image-generator'},
 {title:'Tell a story in motion',description:'Think in scenes, compare available controls and choose the video model for your idea.',href:'/ai/kling'},
 {title:'Develop a product concept',description:'Explore a setting, a material and the visual language around a product.',href:'/workflows/product-campaign'},
 {title:'Make room for sound',description:'Explore the music tools for the mood and pace of a creative project.',href:'/app/music'},
 {title:'Put a voice to the idea',description:'Review the available speech tools and find the right starting point for your text.',href:'/app/speech'},
 {title:'Work through a campaign',description:'Use a guide to connect the brief, visual experiments and review stages.',href:'/ai/marketing'},
 {title:'Learn a new approach',description:'Build a workflow from short lessons and practical questions.',href:'/academy/courses/epifera-essentials'},
 {title:'Keep the next idea moving',description:'Return to your conversations and created files when the next direction arrives.',href:'/app'},
 ],action:'Explore a reference campaign story'},
 assurance:{title:'Understand the tools, access and terms',description:'Start with what is available today, and keep planned team capabilities distinct.',action:'Explore team planning',items:[
 {title:'Your account',description:'Use the existing sign-in methods and review your profile and account settings.',href:'/ai/docs/account-management',icon:'text'},
 {title:'Clear payment terms',description:'Review the actual offer, payment and refund conditions before purchasing.',href:'/legal/paid-access',icon:'text'},
 {title:'Data and privacy',description:'Read the published privacy policy for the service and its handling of data.',href:'/legal/privacy-policy',icon:'spark'},
 {title:'Created files',description:'Review your outputs and download the available results. Generated files have a two-calendar-month storage period unless deleted earlier.',href:'/app/files',icon:'image'},
 {title:'Team requirements',description:'Shared access, company controls and team capacity remain planned capabilities to discuss.',href:'/business',icon:'spark'},
 {title:'Help when you need it',description:'Use the guides or contact support with a clear description of your question.',href:'/ai/docs',icon:'voice'},
 ]},teamsAction:'Explore team planning',
 story:{label:'The creative process',title:'From the first idea to the next project',items:[
 {title:'Explore the tools',text:'Find the creative area that fits the idea you want to develop.',href:'/ai',action:'Discover the tools'},
 {title:'Choose a model',text:'Review the models, inputs and controls available in your workspace.',href:'/app/models',action:'Explore the catalog'},
 {title:'Shape the brief',text:'Write down the subject, direction and constraints that matter to the work.',href:'/app',action:'Open the workspace'},
 {title:'Review the cost',text:'Check the displayed task price and the relevant payment terms before starting paid work.',href:'/legal/paid-access',action:'Read payment terms'},
 {title:'Create a first direction',text:'Use the existing tools to develop the next image, conversation or media task.',href:'/app/image',action:'Explore image creation'},
 {title:'Review and download',text:'Compare available results in My files and keep what is useful for the project.',href:'/app/files',action:'Open My files'},
 {title:'Keep learning',text:'Use the guides and lessons to refine the way you approach the next idea.',href:'/academy/courses/epifera-essentials',action:'Explore the lessons'},
 ]},
 values:{label:'What guides the work',title:'What we believe',items:[
 {title:'Your idea comes first',text:'The tools should give your next idea somewhere to go. Start with what you want to express, then explore the possibilities around it.'},
 {title:'Know the next step',text:'A useful workflow makes the choices clear: the model, the brief, the settings and the cost before a paid task begins.'},
 {title:'Keep the work connected',text:'Conversations, references and outputs belong to the same creative process. It should be easy to find the useful parts again.'},
 {title:'Learn by trying',text:'A first result is a direction to review. Use small experiments, useful references and clear feedback to shape what comes next.'},
 ]},
 products:{label:'Our tools',title:'Choose where your next idea begins',reference:'Magnific interface reference — layout illustration',items:[{name:'AI Video Generator',href:'/app/models'},{name:'AI Image Generator',href:'/ai/image-generator'},{name:'Music Generator',href:'/app/music'},{name:'Text to Speech',href:'/app/speech'},{name:'AI Chat',href:'/app'},{name:'Workflow guides',href:'/workflows/product-campaign'},{name:'MCP concept',href:'/mcp'},{name:'Model catalog',href:'/app/models'},{name:'Visual references',href:'/photos/nature'}]},
 models:{title:'A choice of models. One workspace.',description:'Explore model families and their available inputs, settings and costs in the catalog.',label:'Explore AI model families',note:'Preview media from Magnific. Illustrations do not promise a particular generated result.'},
 faqTitle:'Frequently asked questions',faq:[
 {question:`What is ${brandName}?`,answer:`${brandName} is a creative workspace for images, video, music, speech and chat. Explore models, develop an idea and review the available results in your account.`},
 {question:`Where should I start?`,answer:'Explore the tools overview or open the workspace. Choose a creative area and review the available models and controls.'},
 {question:'Can I create AI videos?',answer:'Video models are available in the catalog. Their supported inputs, settings and costs are shown in the workspace.'},
 {question:'Can I use the tools for a creative project?',answer:'Explore the available tools and review the terms for your intended use. This page does not add commercial rights or guarantees beyond the actual service terms.'},
 {question:'Does Epifera support company teams?',answer:'A business setup is planned. The team page helps you explore the concept and discuss requirements; it does not activate shared access or a company balance.'},
 {question:'Are API and MCP connections available?',answer:'The MCP page describes a planned connection concept. This page does not provide a live API endpoint or connector access.'},
 {question:'How is my data handled?',answer:'Read the published privacy policy and user agreement for the actual data-processing terms. Contact support if you need clarification about a specific use.'},
 {question:'How long are generated files stored?',answer:'Generated files have a two-calendar-month storage period from their original creation time, unless you delete them earlier. Keep a download of results you need to retain.'},
 {question:'Who owns the content I create?',answer:'Review the user agreement and the terms for the tools and content involved before publishing or delivering a result.'},
 {question:'How can I contact Epifera?',answer:'Write to support@epifera.com for general questions, support, press enquiries or collaboration ideas. The Help Center also contains guides for common tasks.'},
 ],
 contribution:{label:'Build with us',title:'Have an idea for the next step?',description:'A useful question, a workflow suggestion or a collaboration idea — tell us what you have in mind.',action:'Share an idea'},
 contacts:{label:'Get in touch',title:"Let's talk",items:[{title:"What's on your mind?",description:"We'd like to hear from you.",display:'support@epifera.com',href:'mailto:support@epifera.com?subject=Hello%20Epifera'},{title:'Press and partnerships',description:'For press enquiries and collaboration ideas.',display:'support@epifera.com',href:'mailto:support@epifera.com?subject=Epifera%20press%20enquiry'},{title:'Support',description:'Guides, answers and practical next steps.',display:'Open the Help Center',href:'/ai/docs'}]},
 sourceAction:'About the visual references',sourceTitle:'About this page and its references',sourceLink:'View the Magnific reference',sourceNote:'The layout and attributed reference visuals come from the saved Magnific about page. Epifera does not claim its staff, customers, reviews, company history or certifications. The information and contact links here describe Epifera.',
 controls:{close:'Close',pricing:'Pricing',previous:'Previous',next:'Next',models:'AI model preview cards',explore:'Explore',reference:'Magnific visual reference'},
} as const;
