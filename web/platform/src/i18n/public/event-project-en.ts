import {brandName} from '@/config/brand';

export type EventProject = {
  readonly id: string;
  readonly name: string;
  readonly status: 'concept';
  readonly startsAt: string | null;
  readonly endsAt: string | null;
  readonly venue: string | null;
  readonly registrationUrl: string | null;
  readonly speakers: readonly {name: string; role: string}[];
};
export const eventProject: EventProject = {
  id: 'creative-sessions', name: 'Creative Sessions', status: 'concept',
  startsAt: null, endsAt: null, venue: null, registrationUrl: null, speakers: [],
};
const contact = 'mailto:support@epifera.com?subject=Creative%20Sessions';

export const eventProjectEn = {
  metadata: {title: `Creative Sessions — A project by ${brandName}`, description: 'A project concept connecting film, creative conversations and the tools behind the work. Explore the programme, a reference trailer and practical reading.'},
  skip: 'Skip to the project', announcement: 'A workspace for your next idea', status: 'Project concept',
  breadcrumbs: {label: 'Breadcrumb', home: 'Home', projects: 'Projects'},
  hero: {
    label: 'The idea comes first.', title: 'A film, a conversation, and your next creative direction.',
    description: `Explore Creative Sessions, a project concept from ${brandName} for people who care about the craft behind the work.`,
    primary: 'Explore the programme', secondary: 'Project details',
  },
  programme: {
    label: 'The programme', title: 'Three ways into the creative process',
    description: 'A programme preview: watch closely, exchange ideas and explore what the tools can bring to your own work.',
    items: [
      {title: 'A film to start the conversation', meta: 'Screening reference · Candela', description: 'Explore a short film reference and look at the choices behind a scene, a sound and a performance.', href: '#screening', icon: 'video'},
      {title: 'A conversation about craft', meta: 'Discussion prompts · Creative control', description: 'Think through authorship, trust and the decisions that make a piece of work feel intentional.', href: '#conversation', icon: 'voice'},
      {title: 'A meeting of ideas', meta: 'Meet-up concept · Open exchange', description: 'Bring a question, a workflow or a possible collaboration. The next useful idea can start with a conversation.', href: '#meetup', icon: 'spark'},
    ],
  },
  film: {
    label: 'A film reference · Myriam Mira / Magnific', title: 'Candela',
    description: 'A film reference for a conversation about creative direction. Look beyond the tools and notice the decisions that hold an intimate story together.',
    action: 'Watch the trailer', dialog: 'Candela — reference trailer',
    credit: 'Candela by Myriam Mira / Magnific. This is the saved 18.5-second reference trailer, not an Epifera production.',
    notes: [
      'Watch the rhythm of the scene: a gesture, a pause, the space around a character. Which decisions carry the emotion?',
      'Consider the relationship between image and sound. What would you keep, change or develop in your own creative process?',
    ],
  },
  discussion: {
    label: 'Creative conversation', title: 'What stays yours when AI joins the process?',
    meta: 'Four conversation starters',
    description: 'A useful discussion begins with the choices in the work. These prompts are a starting point for the project, with contributors and a date still to be announced.',
    action: 'Discuss the project', href: contact,
    items: [
      {title: 'Creative direction', description: 'Where does the brief end and the next creative decision begin?'},
      {title: 'Authorship', description: 'How do you recognise your own point of view in the result?'},
      {title: 'Trust and context', description: 'What should the audience understand about how the work was made?'},
      {title: 'From experiment to delivery', description: 'Which review steps turn an interesting result into useful work?'},
    ],
  },
  features: [
    {id: 'setting', image: 'cabana', label: 'A setting for the exchange', title: 'A place for creative conversations', meta: 'Venue inspiration · A reference setting', description: 'A good setting leaves room to watch, ask questions and compare approaches. The location for Creative Sessions has not been announced.', action: 'Talk about a setting', href: contact, credit: 'Cannes Lions venue illustration from Magnific. Not an Epifera venue.'},
    {id: 'meetup', image: 'happy-hour', label: 'A meeting of ideas', title: 'Make space for the next idea', meta: 'Meet-up concept · No date announced', description: 'An open exchange around projects, experiments and useful questions. If you have a collaboration idea, tell us what you would like to explore.', action: 'Share a collaboration idea', href: contact, credit: 'Magnific meet-up illustration. Not a photograph of an Epifera event.'},
    {id: 'workflow', image: 'enterprise-spaces', label: 'The tools behind the work', title: 'Keep the creative direction yours', meta: 'Explore the workflow guide', description: 'Move from a brief to visual experiments and review. Use the existing project guide to plan the next steps in your own workspace.', action: 'Explore the workflow', href: '/workflows/product-campaign', credit: 'Magnific interface illustration. Explore the Epifera guide through the link.'},
  ],
  journal: {
    label: 'The project journal', title: 'Ideas worth revisiting', description: 'A reading list for the questions that continue after the first conversation.',
    category: 'Reading list', cardTitle: 'Start with a clear creative brief',
    text: 'Explore the guide to image prompts: intent, subject, composition and the details that matter to your next experiment.', action: 'Read the guide', href: '/blog/ai-art-prompts',
  },
  faqTitle: 'Creative Sessions FAQ',
  faq: [
    {question: 'What is Creative Sessions?', answer: `Creative Sessions is a project concept from ${brandName} connecting a film reference, conversations about creative work and practical exploration of the tools.`},
    {question: 'When and where will it take place?', answer: 'Dates, a venue and contributors have not been announced. This page presents the project concept and programme preview.'},
    {question: 'Can I register for a session?', answer: 'Registration is not open. Project details explain the current status and link to support@epifera.com for questions or collaboration ideas.'},
    {question: 'Is Candela an Epifera film?', answer: 'No. Candela is the Myriam Mira / Magnific film shown here as a credited reference for discussion. The video is the saved 18.5-second trailer.'},
    {question: 'Where can I explore the tools?', answer: 'Open the tools overview or the workspace to review the models, available controls and displayed costs. The workflow guide helps you plan a creative project.'},
  ],
  closing: {title: 'Bring your next idea', description: 'A question about craft, an experiment or a possible collaboration. Start the next conversation with us.', action: 'Explore the project details'},
  details: {title: 'Creative Sessions — project details', description: 'The project is at concept stage. Dates, venue, speakers and registration have not been announced. You can explore the programme preview and contact the project with a question or collaboration idea.', contact: 'Write to support@epifera.com', href: contact, tools: 'Explore the tools'},
  source: {action: 'About the visual references', title: 'About this project and its references', description: 'The layout and credited film, venue, meet-up and interface illustrations come from the saved Magnific Cannes Lions page. They are visual references for this local Epifera concept. Epifera does not claim a Cannes Lions partnership, a confirmed event or the original production.', link: 'View the Magnific reference', href: 'https://www.magnific.com/events/cannes-lions'},
  controls: {close: 'Close', pricing: 'Pricing', reference: 'Magnific visual reference'},
} as const;
