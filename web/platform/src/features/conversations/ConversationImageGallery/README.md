# Conversation images

`ConversationImageGallery` owns the selected artifact and gallery for one conversation.
`ConversationAssistantMessage` renders shared `FileCard` components outside the Markdown
document; opening a card uses the same `FilePreviewDialog` as the file library.

Messages may include `images: Array<{ job: ImageJob; artifact: ImageArtifactMetadata }>`.
The parser accepts only completed image jobs and the existing safe metadata fields.
Artifact bytes are addressed through `/web/v1/image-artifacts/{id}`. The gallery never
queries the global file list. Metadata must come from an authorized API response,
not from model-authored Markdown. Messages without attachments retain their existing
text/Markdown behavior.

The local preview chat references the library's existing job and artifact objects.
The production Go conversation endpoint currently creates text jobs and does not
emit image attachments; production image generation in conversations is separate
backend work. This change adds the client contract and the shared preview UI, not
a new provider call or paid generation flow.

Images are ordered by message sequence and attachment order, and deduplicated by
artifact ID. On the first open, earlier conversation pages are fetched in batches
of 100 if necessary, without inserting them into the visible history. An error
keeps the card available for retry; incomplete galleries are not silently shown.
Requests are cancelled when the conversation unmounts. The history parent is keyed
by conversation ID, so another conversation cannot retain the previous gallery.

The shared preview hides its thumbnail rail and arrows for a single image. Closing
returns keyboard focus to the original card. Markdown images and download links
for structured attachments are omitted to avoid rendering the same image twice.
