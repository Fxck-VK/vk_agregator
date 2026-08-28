# Graphite Brand Palette Design

## Goal

Перевести NeiroHub с визуально доминирующего синего и широкого сиреневого canvas на почти монохромную графитовую основу. Фирменные violet, blue и pink остаются редкими акцентами, а не крупными заливками.

Изменение касается только цвета. Типографика Geist, размеры, отступы, скругления, расположение элементов, тексты, иконки и поведение компонентов не меняются.

## Chosen approach

Использовать централизованные семантические CSS-токены и привязать к ним основные слои интерфейса. Это сохраняет единую тему и позволяет менять палитру без прямой замены цветов в каждом компоненте.

Не используются два альтернативных подхода:

- массовая замена hex-значений в компонентах — создаёт несогласованные роли цвета;
- полный рефакторинг дизайн-системы — выходит за рамки задачи и затрагивает лишние компоненты.

## Color palette

| Role | Dark | Light |
| --- | --- | --- |
| Global background / outer canvas | `#0C0C0F` | `#F7F7FA` |
| Main workspace | `#111217` | `#FFFFFF` |
| Panels and sidebar | `#15161C` | `#F3F3F7` |
| Cards and inputs | `#1A1B22` | `#EEEEF4` |
| Hover and elevated surfaces | `#20212A` | `#E8E8F0` |
| Borders | `#2A2B35` | `#DEDFE7` |
| Primary text | `#F5F5F7` | `#17171B` |
| Secondary text | `#9B9DA8` | `#6B6C76` |
| Brand violet | `#9A7CF5` | `#7563E6` |
| Brand blue | `#7C8FF7` | `#6678E6` |
| Brand pink | `#F09AF0` | `#D56DD9` |
| Focus | `#A9CFFF` | `#8475F0` |

The main brand gradient is exact and theme-independent:

```css
linear-gradient(
  120deg,
  #f29af3 0%,
  #b983f6 48%,
  #7c8ff7 100%
)
```

Existing semantic danger, success, warning and information colors remain unchanged unless they are aliases of the removed generic blue accent. Their meaning must stay recognisable in both themes.

## Token architecture

Global theme tokens will distinguish layers that are currently collapsed into `surface` and `surface-raised`:

- `--color-background`: page background and outer canvas;
- `--color-workspace`: main right-hand workspace;
- `--color-panel`: sidebar and persistent panels;
- `--color-surface`: cards and input surfaces;
- `--color-surface-raised`: hover, popover and elevated surfaces;
- `--color-border`, `--color-text`, `--color-text-muted`;
- `--color-brand-violet`, `--color-brand-blue`, `--color-brand-pink`;
- `--color-accent` and `--color-accent-strong` remain compatibility aliases to the brand palette;
- `--color-focus`;
- `--gradient-brand`.

Both explicit light/dark themes and `data-theme="system"` must resolve to the same semantic roles.

## Component mapping

- `AppShell` outer canvas changes from `#9494F8` to `--color-background`.
- The main right-hand workspace uses `--color-workspace`.
- Sidebar and persistent panels use `--color-panel`.
- Cards, inputs and ordinary control surfaces use `--color-surface`.
- Hover, selected neutral, menus and elevated surfaces use `--color-surface-raised`.
- Ordinary headings and body copy remain neutral.
- Solid primary controls remain non-gradient and use an accessible foreground color.
- The brand gradient is limited to compact brand accents such as the logo mark, selected-model indicator, balance/token indicator or similarly small premium markers. It must not become a page background, card background or large primary button fill.
- The word `нейросетей` in a hero title may use the brand gradient when that title is present; other H1/H2 text stays `--color-text`.

## Accessibility and states

- Text/background combinations must preserve readable contrast.
- Focus indicators use the dedicated focus token and remain visible in both themes.
- Status colors retain their semantic meaning.
- Disabled, hover, active and selected states remain distinguishable without relying only on pink or violet.

## Verification

1. Contract tests assert every dark and light palette value, the exact gradient and the new semantic layer tokens.
2. A shell style test asserts that `#9494F8` is absent and the outer canvas/main workspace use their semantic tokens.
3. Repository search confirms the retired values `#2187FF`, `#0F6FDB` and `#9494F8` are not used as core brand/canvas colors.
4. Existing Vitest, asset, lint, typecheck, build and packaging checks pass.
5. Local browser verification covers dark/light themes and desktop/mobile sizes, with special attention to the right-hand workspace background and the narrow gap around panels.

## Out of scope

- Layout, spacing, sizes and border radii;
- typography;
- image and illustration artwork;
- content changes;
- redesigning status colors;
- changing application behavior or navigation.
