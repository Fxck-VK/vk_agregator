# Compact Conversation Composer Design

## Goal

Make the active conversation composer match the approved compact reference: one rounded surface with the prompt, media control, and circular send action inside it, plus a centered note below.

## Structure

- Keep the accessible label but hide the visible `Новое сообщение` heading.
- Use one outer rounded composer surface instead of an outer card plus a separately bordered textarea.
- Place the media control at the bottom left of the surface.
- Place a circular arrow submit control at the bottom right of the surface.
- Show a compact pricing/disclaimer line below the surface.

## Behavior

- Preserve Enter to send and Shift+Enter for a new line.
- Preserve disabled, pending, auto-scroll, and typing-indicator behavior.
- Keep media upload disabled until the message API supports attachments; the control is present without pretending that an unsupported upload will be sent.
- Do not display an invented numeric price; state that the price depends on the selected model until verified chat pricing is available from the backend.

## Non-goals

- No backend or billing changes.
- No attachment submission flow.
- No changes to chat history, message cards, or workspace navigation.
