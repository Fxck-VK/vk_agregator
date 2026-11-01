# Permanent public account ID

Status: approved behavior from the owner's acceptance of the public-ID proposal.

Each canonical account receives one database-assigned positive sequence number.
The server formats it as `EP-000123` (at least six decimal digits; longer numbers
expand without truncation). The UUID remains the internal owner/session key.
The number belongs to the account, not an email or OAuth identity. Existing
accounts receive numbers in additive migration000065; subsequent inserts use the
identity default. Numbers are unique and are never allocated by a profile read.
A database trigger prevents changes; schema down is blocked so rollback cannot
remove or later reassign published identifiers.

The existing authenticated account profile gains optional `public_id` for
rolling compatibility. Production wiring supplies the repository; missing rows
or storage errors do not invent IDs. Local preview uses a clearly synthetic ID.
The existing profile identity card shows the ID and an explicit copy button,
with honest pending/success/failure states and selectable text on clipboard
failure. Preserve the card, header, navigation and all unrelated presentation.

The existing guarded operator user lookup accepts an exact public ID using
`public_id`; legacy UUID `user_id` lookup remains. Mixed filters are rejected.
The result projects only public ID, a sanitized account reference and existing
nonpersonal account metadata. It supports browser-only accounts without a VK
user row. No public account-search API, account mutation, identity merge,
authorization, billing or provider change is included.

Validation: format/parse boundaries; unique concurrent assignment; existing-row
migration and later inserts; profile stability across identity changes; unknown
and foreign lookup/auth guards; copy success/failure; current profile rendering;
focused Go/admin/platform checks. Check the affected profile at360/390/768/1024/
1440/1920/2560/3840 CSS px, including QHD2560x1440 and UHD3840x2160. Record actual
visual evidence and any tool limitation. No commit, push or deployment is requested.
