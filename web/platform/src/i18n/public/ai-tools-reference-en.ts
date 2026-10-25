// Reference 02 artwork and directory structure. Active actions use Epifera routes; planned entries are previews.
export const aiToolsReferenceEn = {
  "hero": {
    "file": "hero.webp",
    "alt": "",
    "width": 2000,
    "height": 1125
  },
  "categories": [
    {
      "id": "image",
      "label": "Image",
      "tone": "blue",
      "tools": [
        {
          "title": "Assistant",
          "description": "Explore an idea and shape your creative brief with AI.",
          "icon": "assistant",
          "href": "/app"
        },
        {
          "title": "Image Editor",
          "description": "Describe an edit with the available image models.",
          "icon": "image-editor",
          "href": "/app/image"
        },
        {
          "title": "Image Generator",
          "description": "The image in your head. Now on screen.",
          "icon": "images",
          "href": "/ai/image-generator"
        },
        {
          "title": "Image Upscaler",
          "description": "Every detail, exactly as you intended. At full resolution.",
          "icon": "upscale",
          "planned": true
        },
        {
          "title": "Cinematic Shot",
          "description": "Explore a cinematic direction for your next image.",
          "icon": "video-timeline",
          "href": "/app/image"
        },
        {
          "title": "Variations",
          "description": "Explore a different direction from an image reference.",
          "icon": "duplicate-new",
          "href": "/app/image"
        },
        {
          "title": "Character Generator",
          "description": "Explore character concepts from a prompt or reference.",
          "icon": "user",
          "href": "/app/image"
        },
        {
          "title": "Skin Enhancer",
          "description": "The detail is there. This makes it visible.",
          "icon": "skinenhancer",
          "planned": true
        },
        {
          "title": "Change Camera",
          "description": "Describe a new angle with an image-editing model.",
          "icon": "camera",
          "href": "/app/image"
        },
        {
          "title": "Mockup Generator",
          "description": "Create a product concept with an image-generation model.",
          "icon": "mockup",
          "href": "/app/image"
        },
        {
          "title": "Remove Background",
          "description": "One click. Clean subject.",
          "icon": "remove-bk",
          "planned": true
        },
        {
          "title": "Relight",
          "description": "Describe the lighting you want to explore.",
          "icon": "effects",
          "href": "/app/image"
        }
      ]
    },
    {
      "id": "video",
      "label": "Video",
      "tone": "green",
      "tools": [
        {
          "title": "Video generation",
          "description": "Find a video model for your next scene.",
          "icon": "video",
          "href": "/app/models"
        },
        {
          "title": "Video with audio",
          "description": "Explore video models and their available audio settings.",
          "icon": "volume",
          "href": "/app/models"
        },
        {
          "title": "Video editing",
          "description": "Edit, trim and restyle footage in seconds.",
          "icon": "video-editor",
          "planned": true
        },
        {
          "title": "Scene stitching",
          "description": "Concatenate scenes into a finished sequence.",
          "icon": "video-timeline",
          "planned": true
        },
        {
          "title": "Lip sync",
          "description": "Match generated speech to any character's mouth.",
          "icon": "lipsync",
          "planned": true
        },
        {
          "title": "Motion control",
          "description": "Direct camera moves and pacing for every shot.",
          "icon": "motioncontrol",
          "planned": true
        },
        {
          "title": "Style transfer",
          "description": "Apply a consistent look across an entire clip.",
          "icon": "palette",
          "planned": true
        },
        {
          "title": "Video upscaling",
          "description": "Boost resolution and detail on existing footage.",
          "icon": "upscale",
          "planned": true
        }
      ]
    },
    {
      "id": "audio",
      "label": "Audio",
      "tone": "teal",
      "tools": [
        {
          "title": "Soundtracks",
          "description": "Create music from a mood, a story or a simple prompt.",
          "icon": "volume",
          "href": "/app/music"
        },
        {
          "title": "Voiceover",
          "description": "Give your words a voice with the available speech models.",
          "icon": "microphone",
          "href": "/app/speech"
        },
        {
          "title": "Sound effects",
          "description": "Explore sound creation with the available studio tools.",
          "icon": "sound-effect",
          "href": "/app/music"
        },
        {
          "title": "Audio cleanup",
          "description": "Clean up and master your audio in one click.",
          "icon": "clean",
          "planned": true
        },
        {
          "title": "Music remix",
          "description": "Explore the available ways to continue and reshape music.",
          "icon": "shuffle",
          "href": "/app/music"
        },
        {
          "title": "Dubbing",
          "description": "Translate and revoice content into new languages.",
          "icon": "translate",
          "planned": true
        },
        {
          "title": "Audio sync",
          "description": "Align sound and dialogue to your timeline.",
          "icon": "refresh",
          "planned": true
        },
        {
          "title": "Mastering presets",
          "description": "Apply pro-grade mixes with a single preset.",
          "icon": "settings",
          "planned": true
        }
      ]
    },
    {
      "id": "spaces",
      "label": "Spaces",
      "tone": "purple",
      "tools": [
        {
          "title": "Reusable workflows",
          "description": "Save a workflow once and run it whenever you need it.",
          "icon": "grid",
          "planned": true
        },
        {
          "title": "Project canvas",
          "description": "Organize projects and assets in a shared canvas.",
          "icon": "apps-spaces",
          "planned": true
        },
        {
          "title": "Team collaboration",
          "description": "Collaborate with your team in real time.",
          "icon": "users",
          "planned": true
        },
        {
          "title": "Asset library",
          "description": "Keep your generated files close at hand.",
          "icon": "collection",
          "href": "/app/files"
        },
        {
          "title": "Version history",
          "description": "Roll back to any earlier version of a project.",
          "icon": "history",
          "planned": true
        },
        {
          "title": "Shared templates",
          "description": "Start from team templates to keep work on-brand.",
          "icon": "template",
          "planned": true
        },
        {
          "title": "Comments & feedback",
          "description": "Leave notes and review work directly on the canvas.",
          "icon": "comment",
          "planned": true
        },
        {
          "title": "Roles & permissions",
          "description": "Control who can view, edit and publish each space.",
          "icon": "shield",
          "planned": true
        }
      ]
    },
    {
      "id": "design",
      "label": "Design",
      "tone": "pink",
      "tools": [
        {
          "title": "Logo Maker",
          "description": "Generate logos and brand marks",
          "icon": "icons",
          "planned": true
        },
        {
          "title": "Vectorizer",
          "description": "Turn images into clean vectors",
          "icon": "vector",
          "planned": true
        },
        {
          "title": "Color Palette",
          "description": "Build palettes from any image",
          "icon": "palette",
          "planned": true
        },
        {
          "title": "Mockups",
          "description": "Preview designs on real products",
          "icon": "mockup",
          "planned": true
        }
      ]
    },
    {
      "id": "3d",
      "label": "3D",
      "tone": "orange",
      "tools": [
        {
          "title": "Image to 3D",
          "description": "Turn images into 3D models",
          "icon": "3d",
          "planned": true
        },
        {
          "title": "3D Scene",
          "description": "Compose and render 3D scenes",
          "icon": "3d-scene",
          "planned": true
        },
        {
          "title": "Texturing",
          "description": "Generate textures and materials",
          "icon": "pattern",
          "planned": true
        },
        {
          "title": "3D Animation",
          "description": "Animate your 3D assets",
          "icon": "animate",
          "planned": true
        }
      ]
    },
    {
      "id": "flows",
      "label": "Flows",
      "tone": "purple",
      "tools": [
        {
          "title": "Automations",
          "description": "Chain tools into one workflow",
          "icon": "shuffle",
          "planned": true
        },
        {
          "title": "Batch Run",
          "description": "Run a flow over many inputs",
          "icon": "squares",
          "planned": true
        },
        {
          "title": "Flow Templates",
          "description": "Start from ready-made flows",
          "icon": "template",
          "planned": true
        },
        {
          "title": "Scheduling",
          "description": "Run flows on a schedule",
          "icon": "clock-timer",
          "planned": true
        }
      ]
    },
    {
      "id": "connections",
      "label": "Connections",
      "tone": "blue",
      "tools": [
        {
          "title": "Integrations",
          "description": "Connect Epifera to your stack",
          "icon": "api",
          "planned": true
        },
        {
          "title": "Imports",
          "description": "Bring an image reference into your creative workspace.",
          "icon": "upload",
          "href": "/app/image"
        },
        {
          "title": "Exports",
          "description": "Download your files for the next stage of your workflow.",
          "icon": "download",
          "href": "/app/files"
        },
        {
          "title": "Webhooks",
          "description": "Trigger actions on events",
          "icon": "code",
          "planned": true
        }
      ]
    }
  ],
  "outcomes": [
    {
      "file": "scenario-1.webp",
      "alt": "Creative spaces",
      "width": 1200,
      "height": 1200,
      "title": "Creative spaces"
    },
    {
      "file": "scenario-2.webp",
      "alt": "Campaign kits",
      "width": 1200,
      "height": 1200,
      "title": "Campaign kits"
    },
    {
      "file": "scenario-3.webp",
      "alt": "Consistent assets",
      "width": 1608,
      "height": 1200,
      "title": "Consistent assets"
    },
    {
      "file": "scenario-4.webp",
      "alt": "End-to-end output",
      "width": 2000,
      "height": 1115,
      "title": "End-to-end output"
    },
    {
      "file": "scenario-5.webp",
      "alt": "Team handoff",
      "width": 1200,
      "height": 1200,
      "title": "Team handoff"
    }
  ],
  "showcase": [
    {
      "file": "tool-1.webp",
      "alt": "Spaces",
      "width": 805,
      "height": 1200,
      "title": "Spaces"
    },
    {
      "file": "tool-2.webp",
      "alt": "AI image generator",
      "width": 758,
      "height": 1200,
      "title": "AI image generator"
    },
    {
      "file": "tool-3.webp",
      "alt": "AI video generator",
      "width": 758,
      "height": 1200,
      "title": "AI video generator"
    },
    {
      "file": "tool-4.webp",
      "alt": "AI Assistant",
      "width": 758,
      "height": 1200,
      "title": "AI Assistant"
    },
    {
      "file": "tool-5.webp",
      "alt": "Image Editor",
      "width": 758,
      "height": 1200,
      "title": "Image Editor"
    },
    {
      "file": "tool-6.webp",
      "alt": "Video Editor",
      "width": 758,
      "height": 1200,
      "title": "Video Editor"
    },
    {
      "file": "tool-7.webp",
      "alt": "Image Upscaler",
      "width": 758,
      "height": 1200,
      "title": "Image Upscaler"
    },
    {
      "file": "tool-8.webp",
      "alt": "Video Upscaler",
      "width": 758,
      "height": 1200,
      "title": "Video Upscaler"
    },
    {
      "file": "tool-9.webp",
      "alt": "Text to Speech",
      "width": 758,
      "height": 1200,
      "title": "Text to Speech"
    },
    {
      "file": "tool-10.webp",
      "alt": "Background Remover",
      "width": 758,
      "height": 1200,
      "title": "Background Remover"
    },
    {
      "file": "tool-11.webp",
      "alt": "Expand",
      "width": 758,
      "height": 1200,
      "title": "Expand"
    },
    {
      "file": "tool-12.webp",
      "alt": "Retouch",
      "width": 758,
      "height": 1200,
      "title": "Retouch"
    },
    {
      "file": "tool-13.webp",
      "alt": "Mockup Generator",
      "width": 758,
      "height": 1200,
      "title": "Mockup Generator"
    },
    {
      "file": "tool-14.webp",
      "alt": "Reimagine",
      "width": 758,
      "height": 1200,
      "title": "Reimagine"
    },
    {
      "file": "tool-15.webp",
      "alt": "Designer",
      "width": 758,
      "height": 1200,
      "title": "Designer"
    },
    {
      "file": "tool-16.webp",
      "alt": "AI icon generator",
      "width": 758,
      "height": 1200,
      "title": "AI icon generator"
    }
  ]
} as const;
