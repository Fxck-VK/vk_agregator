# Guest workspace login entry design

## Goal

Unauthenticated visitors can open `/app` and see the existing NeiroHub landing page without being forced immediately onto the email/password form.

## States

- Authenticated: preserve the current workspace shell, balance, account menu, conversations and page content.
- Refresh required: preserve the neutral session-restoration shell and the delayed top progress indicator.
- Temporarily unavailable: preserve the current safe unavailable state.
- Unauthenticated: render the existing landing page in a guest shell. Do not render the requested private child route.

## Guest shell

- Keep the current sidebar, static header, responsive drawer and desktop collapse behavior.
- Show a `Войти` action in the header trailing area instead of a balance.
- Show a second `Войти` action in the sidebar account slot instead of an email/profile menu.
- Both actions link to the existing `/login` page.
- Do not show conversations, balance, account identity or files.
- Keep the landing page visually unchanged.

## Guest interactions

- The landing prompt remains editable.
- Submitting the prompt as a guest opens `/login` and never calls a private `/web/v1` mutation.
- Workspace destinations presented inside the guest landing and sidebar require login. The authenticated version keeps its existing destinations.

## Security and architecture

- Do not change DEV Basic Auth, backend endpoints, cookie handling or session refresh.
- Do not create a synthetic account, trusted balance or persistent guest identity.
- Guest rendering must not mount `WorkspaceAccountProvider`, receive private session data or render private route children.
- Reuse the current shell and UI tokens; new login controls are React components with CSS modules.

## Verification

- Layout test proves unauthenticated `/app` no longer redirects and omits private child/session data.
- Frame/header/sidebar tests prove the two login actions replace balance/account content.
- Prompt test proves guest submission navigates to login without API mutations.
- Existing authenticated, refresh and unavailable tests remain green.
