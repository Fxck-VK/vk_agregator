# Workspace model selector implementation plan

1. Add failing component tests for loading, opening, search, price display, selection/navigation, keyboard/outside-close behaviour, and catalogue failure.
2. Add failing header tests for all workspace routes and the Inspiration exclusion.
3. Implement an isolated `WorkspaceModelSelector` component with CSS Module and Russian copy.
4. Replace the header title with the selector outside `/app/inspiration`; keep the balance untouched.
5. Run focused tests, then the platform test suite, lint, typecheck, and production build.
6. Review the diff without committing or pushing unless explicitly requested.
