# Changelog

All notable changes to this project will be documented in this file.

## [unreleased]

### Bug Fixes

- Clamp entity list width

- Clamp entity list width to avoid wrapping

- Avoid metadata value type coercion

- Tune diff colors

- Harden scopes, limits, and protocol trust

- Avoid uuid fallback in relationships list

- Clamp entity list width

- Harden approvals, relationships, and audit access

- Sanitize metadata and vault paths

- Require per-item approvals for imports

- Enforce write access and lock entity creates

- Sanitize ui output

- Harden tui sanitization and layout

- Tighten approval access and import validation

- Enforce job ownership in api and mcp

- Enforce access isolation across api and mcp

- Handle invalid input and graph privacy

- Enforce privacy and access controls

- Harden approvals and graph validation

- Validate resource ids across api and mcp

- Validate uuid inputs for approvals and filters

- Add docker hint on db connect failure

- Enforce admin gating for agents and audit

- **server:** Enforce admin boundaries and harden redteam regressions

- **cli:** Sanitize taxonomy/edit headers and clamp narrow boxes

- **cli:** Prevent table grid cells from wrapping

- **cli:** Fit inbox table within box

- **cli:** Correct box sizing and widen layout

- **cli:** Avoid wrapping history filter tokens

- **cli:** Polish table spacing and list readability

- **server:** Harden enrollment validation and expand red-team tests

- **server:** Backfill baseline scopes for existing login users

- **server:** Execute bulk import approvals

- **server:** Seed work log type and surface approval failures

- **cli:** Default add scope to private

- **approvals:** Surface execution failures safely

- **server:** Validate taxonomy before queuing approvals

- **cli:** Polish inbox approval detail and diff formatting

- **server:** Parse job datetime inputs before query execution

- **mcp:** Allow admin-scoped entity updates

- **server:** Harden approval execution and revert executor

- **cli:** Tighten approval UI and relationship visibility

- **cli:** Improve metadata scope rendering and list space

- **server:** Stabilize approval execution and enrich relationship labels

- **cli:** Improve inbox/history readability and jobs batch updates

- **server:** Harden approval enrichment and archived status alias

- **server:** Enrich approval labels for relationship and bulk entity requests

- **cli:** Reduce clipping and surface linked context relationships

- **server:** Handle job relationship lookups and approval regressions

- **cli:** Stabilize tab focus and preserve feedback viewport

- **server:** Raise approval queue cap and normalize audit actors

- **server:** Serialize agent ids in approval payload paths

- **cli,server:** Harden metadata validation and list rendering

- **cli:** Unify filter prompts and relationship detail summaries

- **cli:** Unify palette search flow and enter/esc confirmations

- **cli:** Color scope badges in preview panels

- **cli:** Show full local timestamps in list columns

- **cli:** Upgrade metadata panel and viewport scrolling

- **cli:** Align metadata row indicator with list selection

- **cli:** Copy metadata values without keys

- **server:** Close caller-boundary auth and export regressions

- **cli:** Restore divider helper output

- **cli:** Unify command/search flow and confirm key defaults

- **cli:** Clean tab and mode-line navigation focus

- **cli:** Improve selection contrast and nav focus

- **cli:** Max out selection contrast

- **cli:** Stabilize focus behavior across tabs and settings

- **cli:** Harden metadata interactions and form rendering

- **cli:** Make command palette slash-first with search fallback

- **cli:** Gate row highlights by focus state

- **cli:** Block context detail fetch when item id is missing

- **cli:** Tighten focus transitions and status bar clamping

- **cli:** Keep timestamp columns readable under table shrink

- **cli:** Render context metadata editor preview as table

- **cli:** Reset body scroll when view focus changes

- **cli:** Stabilize tab focus and metadata preview rows

- **cli:** Hide metadata selection checkboxes until active selection

- **cli:** Hide metadata selector column until selection mode is active

- **cli:** Stabilize file and protocol form layouts on resize

- **scripts:** Harden installer fallback and output formatting

- **cli:** Normalize metadata selector visibility and errcheck coverage

- **cli:** Restore metadata active-row indicator without selection

- **cli:** Show metadata active-row indicator in detail/editor views

- **server:** Preserve entity metadata in approval and update flows

- **mcp:** Refresh agent trust state on each tool call

- **server:** Allow archived entity name reuse

- **api:** Normalize entity metadata payloads on create/update

- **api:** Decode double-encoded entity metadata responses

- **server:** Normalize log and file metadata api payloads

- **api:** Harden login baseline taxonomy failures

- **cli:** Show metadata selector column in select mode

- **context:** Merge metadata patches and harden regressions

- **mcp:** Normalize entity metadata in executor responses

- **api:** Use caller scopes for entity list and metadata search

- **auth:** Keep trusted agent mode stable across re-enroll

- **cli:** Surface invalid api key recovery hints

- **cli:** Clamp command help output on narrow terminals

- **cli:** Avoid invalid-key hints on startup 500 errors

- **cli:** Detect api start port conflicts explicitly

- **cli:** Detect coded invalid-key startup errors

- **server:** Preserve job due_at when update omits field

- **cli:** Classify coded multi-api startup conflicts

- **paths:** Remove vault-specific schema and server path assumptions

- **server:** Close api pool when startup enum load fails

- **cli:** Harden startup conflict detection against race

- **cli:** Normalize auth and multi-api api errors

- **cli:** Surface startup conflict and recovery hints

- **cli:** Preserve files edit prefill state

- **cli:** Guard subtask enter without detail

- **cli:** Classify raw startup bind conflicts as multi-api

- **cli:** Detect underscore auth codes in recovery hints

- **cli:** Harden service tail and server-dir edge handling

- **cli:** Harden palette fallback labels for empty rows

- **cli:** Clean up spawned api process on startup failure

- **server:** Harden enum and sanitizer type handling

- **cli:** Block stale lock pid bypass with runtime-state fallback

- **cli:** Harden api error normalization fallbacks

- **cli:** Reject non-positive api lock pids

- **cli:** Stop live api when lock pid is stale

- **cli:** Terminate startup process on conflict failures

- **server:** Normalize enum lookup errors for unhashable input

- **cli:** Normalize lowercase auth error prefixes

- **server:** Harden scope and tag input type validation

- **cli:** Prefer runtime state pid when lock pid conflicts

- **server:** Enforce list-only semantic search kinds

- **server:** Enforce strict kinds type in semantic search

- **server:** Harden context url validators against non-string input

- **server:** Harden context route url payload validation

- **server:** Enforce strict semantic search payload types

- **server:** Harden route tag validators against bad payloads

- **server:** Enforce object-only json coercion for imports

- **server:** Normalize malformed json import errors

- **server:** Harden context url guards for bypassed payloads

- **cli:** Ignore invalid scope entries in metadata badges

- **cli:** Skip non-string metadata scopes in ui parsing

- **cli:** Harden scope parsing for json metadata payloads

- **cli:** Prevent metadata column overflow on narrow widths

- **cli:** Reject object-shaped metadata scope strings

- **cli:** Parse quoted metadata scope strings safely

- **cli:** Reject empty key segments in metadata pipe rows

- **cli:** Preserve pipe chars in metadata scalar values

- **cli:** Treat tab indentation as two metadata spaces

- **cli:** Avoid invalid-key false positives on 403 authz errors

- **cli:** Parse list-shaped api validation errors

- **cli:** Skip trailing query delimiter for empty params

- **cli:** Prevent api lock stealing during startup race

- **cli:** Merge query params on paths with existing query

- **cli:** Keep startup lock when owner is alive

- **server:** Reject duplicate enum rows in registry loader

- **server:** Validate enum row shapes before registry load

- **server:** Reject invalid enum row payload shapes

- **cli:** Detect zombie startup pid as exited

- **cli:** Harden lock/process edge paths and prune dead guards

- **cli:** Harden entities edge branches and prune dead guards

- **cli:** Harden logs view and edit edge branches

- **cli:** Remove unreachable metadata and taxonomy guards

- **cli:** Prune unreachable ui branch guards

- **cli:** Prune unreachable side-by-side fallback branches

- **cli:** Hide ascii banner in non-interactive command output

- **cli:** Disable command ascii banner by default

- **cli:** Resolve server dir from deep home paths

- **cli:** Bound server-dir search for global start

- **cli:** Use unreachable PID in stale lock test for CI compatibility

- **ci:** Use portable parsing for badge generation

- **ci:** Combine server + CLI test counts in badge


### Documentation

- Add cli screenshot

- Link license

- Update readme architecture and legal docs

- Add readme banner asset

- Remove legacy cli image and refresh readme

- **readme:** Switch quickstart to manual setup flow

- **readme:** Move demo section above quickstart

- **readme:** Add quality badges and update demo links

- **readme:** Refresh test badge count


### Features

- **ui:** Arrow-only tab navigation

- Add logs/files/protocols tabs and api

- Restore numeric tab navigation

- Grant admin scope on login

- **db:** Add taxonomy lifecycle fields and active enum filtering

- **server:** Add taxonomy management API and lifecycle guards

- **cli:** Add profile taxonomy management flows

- **mcp:** Add taxonomy management tools

- **search:** Add semantic search across api, mcp, and cli

- **cli:** Add settings tab with api key config

- **cli:** Hardcode default server url

- **server:** Ensure login grants admin scope for user entities

- **cli:** Add startup checks, recovery, quickstart, toasts, and previews

- **cli:** Color startup check statuses

- **cli:** Color toast titles by level

- **cli:** Add table grid layout for inbox

- **cli:** Add inbox preview panel

- **cli:** Improve inbox selection highlight

- **cli:** Show inbox checkboxes only when selecting

- **cli:** Add knowledge list preview layout

- **cli:** Add relationships list preview layout

- **cli:** Add entities list preview layout

- **cli:** Add jobs list preview layout

- **cli:** Add files list preview layout

- **cli:** Add logs list preview layout

- **cli:** Add protocols list preview layout

- **cli:** Add history list preview layout

- **cli:** Add search results preview layout

- **cli:** Add settings keys and agents preview layout

- **cli:** Add taxonomy list preview layout

- **cli:** Add entities history preview layout

- **cli:** Add knowledge link search preview layout

- **cli:** Add relationship create flow preview layout

- **cli:** Add entities relate select preview layout

- **cli:** Improve import export option layout

- **cli:** Improve command palette layout

- **server:** Add mcp-native agent enrollment bootstrap flow

- **cli:** Add reviewer grant overrides for register agent approvals

- **cli:** Show reviewer grant details in approval view

- **cli:** Restore tui onboarding and polish table selection

- **server:** Add mcp auth attach and local insecure mode

- **server:** Scope-based job visibility

- **schema:** Add api and mcp schema contract

- **server:** Add update job and knowledge executors

- **mcp:** Deliver full parity and context bugfix batch

- **cli:** Add selectable metadata panel with yaml copy

- **cli:** Support pipe metadata rows and center hint bar

- **cli:** Extend palette search across core resources

- **cli:** Allow creating relationships with context and jobs

- **cli:** Show structured metadata previews in forms

- **cli:** Ship row-based metadata editor inspect and value copy

- **cli:** Render relationship summaries in grid tables

- **cli:** Add local api lifecycle commands and boxed command output

- **cli:** Add facet entity filters and metadata select mode

- **cli:** Add job detail link actions and metadata path labels

- **scripts:** Add docker bootstrap installer for curl setup

- **cli:** Render command help in nebula boxed layout

- **cli:** Guard api lifecycle with ownership lock

- **release:** Add manifest-driven installer and ci workflows

- **cli:** Redesign detail metadata data view

- **cli:** Add non-interactive api command suite

- **cli:** Redesign diff grid and clarify help output

- **cli:** Add change column and help recipes

- **cli:** Add sectioned diff grid and semantic change colors

- **cli:** Add non-interactive output modes and doctor command

- Replace metadata JSONB with context-of relationships

- **schema:** Add SQLAlchemy models for all 19 database tables


### Infrastructure

- Add CI, pre-commit hooks, linter upgrade, and code conventions

- Add dynamic coverage badges from CI

- Add auto-generated schema docs from SQLAlchemy models

- Add alembic migration management with initial schema

- Add research-backed CLAUDE.md with path-scoped rules and auto-format hook


### Miscellaneous

- Initial nebula-core import

- Fix lint issues

- Scope ruff lint to src

- **server:** Ignore egg-info and sync test deps lockfile

- **repo:** Move artifacts out of nebula-core

- **db:** Set enterprise taxonomy defaults

- **repo:** Remove tracked cli binary artifact

- Checkpoint pending refactor batch before main merge

- **cli:** Drop unused pid reader helper

- **release:** Park github workflows and pause installer

- **installer:** Move runbook into artifacts

- **repo:** Stop tracking artifacts directory files

- Ignore claude worktrees

- Update coverage badges [skip ci]


### Refactoring

- **cli:** Share preview helpers

- **server:** Move inline SQL to QueryLoader query files

- **database:** Move schema snapshot under database folder

- **cli:** Simplify viewport clamping invariants

- **cli:** Remove unreachable metadata width branches

- **cli:** Remove dead metadata table branches

- **cli:** Remove dead rounded-border fallbacks in table grid

- **cli:** Remove dead fallback branches in box renderer

- **cli:** Remove unreachable invalid-key normalization fallback


### Style

- **server:** Satisfy lint wrapping in context metadata merge

- Format nebula_models with ruff


### Testing

- Add cli/api stress coverage

- Fix chaos suite

- Add redteam security regressions

- Add mcp isolation redteam coverage

- Expand redteam approval coverage

- Add redteam concurrency coverage

- Add redteam graph edge cases

- Add write isolation redteam coverage

- Add approvals access control checks

- **redteam:** Expand verified coverage

- **redteam:** Add passing api and integration coverage

- **redteam:** Add api regression xfail coverage

- **redteam:** Add integration regression xfail coverage

- **redteam:** Add graph knowledge privacy coverage

- **redteam:** Add metadata search privacy checks

- **redteam:** Verify job creation agent ids

- **redteam:** Fix api logs list redirect

- **redteam:** Fix api files list redirect

- **redteam:** Verify api history access

- **redteam:** Verify api invalid uuid handling

- **redteam:** Add api agent entity read isolation checks

- **redteam:** Verify knowledge url scheme validation

- **redteam:** Add concurrent entity update check

- **redteam:** Verify mcp invalid uuid handling

- **redteam:** Add graph shortest path private entity check

- **redteam:** Verify api knowledge metadata filtering

- **redteam:** Verify relationship query job isolation

- **redteam:** Verify invalid audit id format rejection

- **redteam:** Verify file attachment isolation

- **redteam:** Verify api relationship query job isolation

- **redteam:** Verify api job query isolation

- **redteam:** Verify api invalid enum handling

- Add redteam coverage for graph invalid uuids

- Cover invalid uuid handling in mcp tools

- Remove xfail for graph file log privacy

- Verify entity history access denial

- Add api invalid uuid coverage

- Cover invalid uuid in mcp resources

- Add invalid uuid coverage for relationships

- Add invalid uuid coverage for job filters

- Add invalid uuid coverage for bulk updates

- Add invalid uuid coverage for api bulk updates

- Add invalid uuid coverage for api keys

- Add invalid uuid coverage for approvals

- Cover invalid uuid in agent update

- Add invalid uuid coverage for audit filters

- Cover invalid scope names in exports

- Cover import invalid format handling

- Add mcp import validation coverage

- Cover invalid entity type in exports

- Format redteam tests

- Stabilize redteam bulk import and isolation tests

- Avoid api jobs redirect

- Accept auth guard on invalid agent id

- Add approval queue rate limit regression

- Cover cli list and table clamp

- Cover inbox bulk approve confirm

- Cover palette sanitization

- Cover sanitize and error clear

- **redteam:** Fix approval queue rate-limit repro payload

- **redteam:** Add cli repros for sanitize and narrow-width overflow

- **redteam:** Add relationship approval gate repro coverage

- **redteam:** Add protocol trust boundary regression repros

- **redteam:** Add taxonomy admin boundary repros

- **redteam:** Add taxonomy cli sanitize repro

- **redteam:** Add builtin scope rename repros

- **redteam:** Tighten export/import validation

- **redteam:** Drop xfails for access control

- **redteam:** Lock taxonomy and mcp hardening

- **server:** Align API route tests with admin access controls

- **server:** Remove stale xfail markers and align audit UUID tests

- Remove deprecated utcnow in redteam log tests

- **server:** Add rate limit and mcp auth negative coverage

- **server:** Add import matrix and files logs isolation coverage

- **server:** Add relationship update approval guard coverage

- **cli:** Add command boot and help smoke coverage

- **cli:** Add files logs protocols taxonomy state machine coverage

- **cli:** Cover exports imports and bulk api wrappers

- **cli:** Cover jobs and knowledge api wrappers

- **cli:** Cover json map parsing and default client base url

- **cli:** Add components box dialog statusbar banner render coverage

- **cli:** Add command palette and selection state machine coverage

- **cli:** Add entities add and bulk flows regression coverage

- **cli:** Add history and search init view regression coverage

- **cli:** Cover app view rendering and tab nav helpers

- **cli:** Add import export state machine coverage

- **cli:** Cover import export view rendering helpers

- **cli:** Add entities view render and search input coverage

- **cli:** Add files view render and add edit coverage

- **cli:** Add history view render coverage

- **cli:** Cover app center block helper

- **cli:** Cover inbox detail and filter rendering

- **cli:** Cover entities add view rendering

- **cli:** Cover jobs list add and edit flows

- **cli:** Cover knowledge add link and edit flows

- **cli:** Cover app quickstart toast relogin and startup classification

- **cli:** Cover entities history confirm and relationship subflows

- **cli:** Expand relationships tab coverage for mode detail edit and confirm

- **cli:** Expand logs add edit tag and timestamp coverage

- **cli:** Expand protocols tag apply and edit coverage

- **cli:** Expand profile keys agents trust and masking coverage

- **cli:** Cover profile taxonomy archive activate and rendering

- **cli:** Add metadata editor unit coverage

- **cli:** Cover knowledge and jobs edit scope paths

- **cli:** Add internal cmd localhost smoke coverage

- **server:** Cover mcp db pool connection error translation

- **server:** Stabilize postgres test db lifecycle

- **server:** Cover approval poison and taxonomy lifecycle

- **server:** Expand relationship and queue-limit coverage

- **server:** Improve relationship isolation and helper coverage

- **server:** Cover approvals error and uuid validation paths

- **server:** Expand helper and relationship route coverage

- **server:** Expand enrollment and relationship route coverage

- **server:** Increase helper coverage for approval and audit paths

- **server:** Close remaining helper branch coverage

- **server:** Expand file and log route coverage

- **server:** Broaden context route validation coverage

- **server:** Expand jobs imports and protocols route coverage

- **server:** Expand entity and context route coverage

- **server:** Extend entity route edge-case coverage

- **server:** Improve export route helper coverage

- **cli:** Cover table-grid focus gating and settings section nav

- **cli:** Cover palette jumps for search resources

- Enforce legacy scope rejection coverage

- **cli:** Add coverage for api service command helpers

- **cli:** Rename default base url command test file

- **server:** Cover post-approval visibility for queued creates

- **server:** Add approval visibility and trust-toggle regressions

- **server:** Register context agents in mcp isolation fixtures

- Expand regressions for trust toggle, lock recovery, and archive reuse

- Harden lifecycle and duplicate-entity api regressions

- Add guardrail coverage for start lock and scope-variant duplicates

- Extend lifecycle log defaults and entity uniqueness variants

- **cli:** Cover api log tail limit rendering

- **server:** Cover approval metadata deep-merge regression

- **cli:** Extend metadata row highlight regressions

- **server:** Cover entity metadata normalization responses

- **server:** Expand entity metadata normalization guardrails

- **server:** Add metadata constraint and response-shape regressions

- **server:** Cover non-object metadata response normalization

- **server:** Harden metadata normalization string-shape cases

- **server:** Extend metadata normalization edge-case coverage

- **server:** Cover entity metadata normalizer edge cases

- **server:** Extend entity metadata normalizer regressions

- **server:** Assert metadata normalizer preserves entity fields

- **server:** Extend entity metadata normalizer stability coverage

- **server:** Verify get-entity scope segment filtering

- **server:** Expand approval metadata redteam coverage

- **server:** Expand redteam preapproval metadata coverage

- **server:** Expand metadata redteam coverage for files and logs

- **server:** Redteam normalize file and log payload paths

- **server:** Cover queued update entity metadata approval

- **server:** Cover login baseline missing status

- **server:** Verify queued entity metadata patch merge

- **cli:** Expand command and metadata coverage

- **cli:** Cover service health and server-dir edge paths

- **cli:** Cover run-start success and startup status branches

- **cli:** Stabilize command API harness and expand search branches

- **server:** Expand admin-gate matrix and fix get-agent input model

- **server:** Expand bootstrap auth gate and approval admin checks

- **cli:** Harden startup recovery and health-check coverage

- **cli:** Extend startup recovery shortcut coverage

- **server:** Add non-admin admin-tool gating matrix

- **server:** Add due_at timezone and transition matrices

- **server:** Cover approval request-type normalization matrix

- **server:** Add approval executor request-type matrix

- **server:** Add migration manifest drift guards

- **server:** Add approvals api request-type matrix

- **server:** Add schema artifact parity checks

- **cli:** Add startup invalid-key reauth recovery flow coverage

- **cli:** Expand app state and client error branch matrices

- **server:** Expand context auth and local-insecure helper coverage

- Add cli palette/cmd and server helper branch matrices

- Expand ui state/search matrices and server helper coverage

- **cli:** Expand relationships create-flow branch matrix

- **cli:** Expand context and relationships branch matrices

- **cli:** Expand entities rel-edit and scope helper branches

- **cli:** Expand entities edit and metadata-copy branches

- **cli:** Expand entities bulk and relationships helper branches

- **cli:** Expand entities add-search-meta helper branches

- **cli:** Expand history and taxonomy helper branch matrix

- **cli:** Add files helper and key-handler branch tests

- **cli:** Extend root command and runTUI branch coverage

- **cli:** Expand inbox helper and diff branch coverage

- **cli:** Expand logs helper and state branch coverage

- **cli:** Add jobs helper and interaction edge coverage

- **cli:** Expand protocols helper and state branch coverage

- **cli:** Expand files helper and save branch coverage

- **cli:** Cover protocols detail and view branch matrix

- **server:** Add audit log uuid filter validation matrix

- **cli:** Expand preview and relationship helper coverage

- **cli:** Add import-export helper branch matrix

- **cli:** Add profile input handler branch tests

- **cli:** Deepen files add-edit handler branch coverage

- **cli:** Add entities filter helper coverage matrix

- **cli:** Add context link helper coverage paths

- **cli:** Cover metadata helper branches in ui and components

- **server:** Expand mcp branch coverage and stabilize test bootstrap timeout

- **server:** Expand context wrapper guard branch coverage

- **server:** Cover bulk update and revert wrapper branches

- **server:** Cover log wrapper approval and visibility branches

- **server:** Expand relationship wrapper edge coverage

- **server:** Expand wrapper and executor branch coverage

- **server:** Cover context and revert wrapper edge branches

- **server:** Add unit coverage for bulk import normalizers

- **server:** Expand file/protocol/agent/taxonomy wrapper coverage

- **server:** Add remaining branch coverage for mcp server wrappers

- **server:** Expand enum and auth context edge-case coverage

- **server:** Cover auth owner and approval edge branches

- **server:** Deepen auth middleware branch coverage

- **server:** Expand protocol and file route edge coverage

- **server:** Lock taxonomy and agent route edge branches

- **server:** Cover import route access and approval branches

- **server:** Close jobs and logs route edge branches

- **server:** Expand route helper edge coverage

- **server:** Deepen context and entity route edge coverage

- **server:** Cover approvals and audit route edge branches

- **server:** Close exports and relationships route coverage gaps

- **server:** Close remaining models and helper coverage gaps

- **cli:** Expand service runtime error-path coverage

- **cli:** Expand metadata helper branch coverage

- **cli:** Deepen metadata editor state-machine coverage

- **cli:** Cover entity filter input state machine branches

- **cli:** Expand search update and view branch coverage

- **cli:** Complete scope helper branch coverage

- **cli:** Harden relationship create render branches

- **cli:** Cover history keypaths and log edit-tag guards

- **cli:** Cover context scope commit edge branches

- **server:** Lock trust persistence and api version

- **server:** Close executor edge-case approval branches

- **cli:** Deepen search branch coverage

- **cli:** Cover audit and health api edge paths

- **cli:** Cover main entrypoint exit paths

- **cli:** Add api wrapper error matrix

- **cli:** Close taxonomy list and wrapper error branches

- **cli:** Cover agent list and interactive login edge paths

- **cli:** Cover root command delegation and terminal detection

- **cli:** Add config read and save error path coverage

- **cli:** Deepen service lock and state edge coverage

- **cli:** Expand service start stop log edge coverage

- **cli:** Close keys command error and empty-state branches

- **cli:** Expand box component edge-branch coverage

- **cli:** Deepen box table and diff wrapping branches

- **cli:** Cover box metadata marshal and title fallback branches

- **cli:** Cover interactive missing-config runTUI branch

- **cli:** Add help flag and subcommand filter edge tests

- **server:** Cover schema contract and export helper module

- **server:** Cover db pool and import extraction branches

- **server:** Add create-context executor edge-path coverage

- **server:** Drive executors unit coverage to 100 percent

- **cli:** Expand import-export branch coverage matrix

- **cli:** Close metadata scalar normalization branches

- **cli:** Harden logs mode and update branch matrix

- **cli:** Expand files list and mode branch coverage

- **cli:** Expand profile update and input branch coverage

- **cli:** Close taxonomy prompt and command error branches

- **cli:** Close app quickstart and arrow helper branches

- **cli:** Cover palette search helper branches

- **cli:** Expand api error normalization matrix

- **cli:** Add app update and view branch matrix

- **cli:** Expand status hints and stabilize conflict probes

- **cli:** Close app palette and tab-nav branch gaps

- **cli:** Cover statusbar width and jobs mode-key branches

- **cli:** Expand inbox approval and grant branch coverage

- **cli:** Add jobs detail and relationship input branch matrix

- **cli:** Add jobs update and view branch matrix

- **cli:** Complete protocols apply-input branch coverage

- **cli:** Add jobs link and status error-path regression matrix

- **cli:** Expand context helper and list-key branch coverage

- **cli:** Add context update and render branch matrix

- **cli:** Complete inline value formatting branch matrix

- **cli:** Expand history view and audit-value branch coverage

- **cli:** Expand files logs and relationship helper branch coverage

- **cli:** Complete relationship filter and mode-line branch matrix

- **cli:** Lock protocols update and edit helper branch matrix

- **cli:** Expand protocols list and add-key branch coverage

- **cli:** Cover logs preview detail and list branches

- **cli:** Cover jobs mode filter list and save-add branches

- **cli:** Cover jobs list render and save-edit branches

- **cli:** Cover jobs preview and logs list branch matrix

- **cli:** Expand jobs edit and logs add-render branch matrix

- **cli:** Expand entities add detail and confirm branch matrix

- **cli:** Add entities list and mode-line branch matrix

- **cli:** Cover entities add render and save validation branches

- **cli:** Expand entities history and relate branch matrix

- **cli:** Cover entities view edit and metadata detail branches

- **cli:** Expand metadata helper branch matrix

- **cli:** Expand inbox branch coverage matrix

- **cli:** Close search history and inbox helper branches

- **cli:** Close relationship confirm and name branches

- **cli:** Expand relationship create and inbox diff branches

- **cli:** Harden metadata inspect and history scope helpers

- **cli:** Raise component and jobs helper branch coverage

- **cli:** Harden relationships list and create branches

- **cli:** Raise profile update and render branch coverage

- **cli:** Close app palette and startup edge branches

- **cli:** Cover quickstart and onboarding key paths

- **cli:** Cover files list preview and detail edge paths

- **cli:** Cover app edge startup and quickstart branches

- **cli:** Cover context list and detail edge branches

- **cli:** Cover metadata sync and block edge branches

- **cli:** Cover json map unmarshal fallback branches

- **cli:** Harden history state and tablegrid edge branches

- **cli:** Cover context loader and history render edge branches

- **cli:** Close history nav and tablegrid fit branches

- **cli:** Extend jobs subtask and formatting branch coverage

- **cli:** Extend jobs scope and link input branch coverage

- **cli:** Cover jobs noop-key and subtask escape branches

- **cli:** Expand metadata wrapping and column width coverage

- **cli:** Harden preview wrapping and scope row branches

- **cli:** Lock preview scope fallback and label clamp paths

- **cli:** Extend metadata grid and preview unicode edge coverage

- **cli:** Harden ui edge-case coverage for list and edit flows

- **cli:** Harden relate rendering and relationship summary edge cases

- **cli:** Cover files add-view mode focus and reset edge cases

- **cli:** Expand metadata parser and plain list edge coverage

- **cli:** Lock app view-state and row-highlight edge branches

- **cli:** Harden relationships list layout and preview edge cases

- **cli:** Harden metadata inspect and relationship create edge paths

- **cli:** Close inbox and history edge-case coverage gaps

- **cli:** Cover audit value formatter edge serialization cases

- **cli:** Cover clipboard command failure branch

- **cli:** Cover logs edit key handling edge paths

- **cli:** Harden context entity and file tag scope commits

- **cli:** Harden history actor reference edge parsing

- **cli:** Cover profile taxonomy selection and render edge cases

- **cli:** Harden context and entity save failure branches

- **cli:** Harden sanitize and approval-type helper edges

- **cli:** Harden profile detail and metadata width edge paths

- **cli:** Harden api client failure-path coverage

- **cli:** Harden metadata editor edge interactions

- **cli:** Harden metadata parser and box edge branches

- **cli:** Harden relationship edit and scope box edge cases

- **cli:** Cover metadata grouping and preview width edges

- **cli:** Harden relationship create-type render edges

- **cli:** Harden relationship create-search edge cases

- **cli:** Harden protocol tag-input edge cases

- **cli:** Harden titled box and metadata table edge cases

- **cli:** Harden history detail and list edge paths

- **cli:** Harden entity scope helper edge cases

- **cli:** Harden context unmarshal compatibility edges

- **cli:** Harden metadata selectable block edge cases

- **cli:** Harden table grid helper edge branches

- **cli:** Harden history tiny-width and blank-actor edges

- **cli:** Harden status bar clamp and tiny-width edges

- **cli:** Harden inbox list view layout edge branches

- **cli:** Harden history scopes and actors render edges

- **cli:** Harden metadata editor service and box edge cases

- **server:** Harden enum registry edge-case coverage

- **cli:** Harden data-view metadata and dialog edge branches

- **server:** Close models sanitizer and validator edge branches

- **cli:** Harden dialog and tablegrid narrow-width branches

- **cli:** Harden metadata parser and preview edge branches

- Expand cli update and server enum/model edge cases

- Cover cli modal and validator edge cases

- Harden multi-api hint and enum metadata edge paths

- **cli:** Harden startup conflict text edge cases

- **cli:** Harden startup reauth edge-case coverage

- **cli:** Cover api lock recovery edge branches

- **cli:** Cover corrupt lock fallback with live runtime state

- **server:** Expand enum validator edge-case matrix

- **server:** Expand context url edge-case regressions

- **cli:** Prevent help path test deadlock on stdout pipe

- **cli:** Harden help rendering edge-case coverage

- **cli:** Cover unreadable startup log detection path

- **cli:** Cover help flag dedupe guard path

- **server:** Lock enum loader trim and error propagation

- **cli:** Cover api lock permission denied branch

- **cli:** Lock entities loader query and error paths

- **cli:** Lock metadata scope normalization edge cases

- **cli:** Cover zombie and missing pid liveness paths

- **cli:** Harden service zombie and stop edge cases

- **cli:** Expand entities history preview edge matrix

- **cli:** Lock stop-process noop and exit edge paths

- **cli:** Cover dead-state stop and eperm liveness branches

- **cli:** Extend service lock and liveness edge coverage

- **cli:** Harden protocols list loading and selection edges

- **cli:** Cover protocol mode and tiny-list rendering edges

- **cli:** Cover stop escalation and ps-empty edge paths

- **cli:** Lock file edit payload and tag-render edge branches

- **cli:** Harden relationships edge-case branch coverage

- **cli:** Lock tiny-width relationship summary rendering

- **cli:** Extend relationships and metadata inspect edge coverage

- **cli:** Extend service dir resolution edge coverage

- **cli:** Harden profile settings and table edge paths

- **cli:** Harden context edit and link-search edge branches

- **cli:** Cover add-metadata parse error branch in logs form

- **cli:** Harden jobs edit render and save error edge paths

- **cli:** Cover entity metadata sync init edge branch

- **cli:** Harden context list edit and detail failure branches

- **cli:** Harden helper fallback and scope-loading edge cases

- **cli:** Harden cmd config and helper edge branches

- **cli:** Stabilize service edge branches and remove test flake

- **cli:** Close resolve-server-dir branch coverage gaps

- **cli:** Close agent history and jobs edge branches

- **cli:** Harden entities detail and add edge branches

- **cli:** Harden inbox update fallback branches

- **cli:** Harden entities add/list/detail edge branches

- **cli:** Harden protocols add and edit error branches

- **cli:** Cover add-form navigation wrap in jobs model

- **cli:** Harden jobs list and status edge branches

- **cli:** Harden entities confirm and relation edit error branches

- **cli:** Harden metadata parser and preview edge branches

- **cli:** Extend metadata edge-case regression matrix

- **cli:** Expand entity filter and line-width edge coverage

- **cli:** Lock entity filter, bulk parse, and metadata copy edges

- **cli:** Harden startup conflict race and bulk parse edges

- **server:** Lock enum fetch failure propagation path

- **cli:** Harden preview wrapping and scope token branches

- **cli:** Harden clipboard fallback and form-grid edge paths

- **cli:** Cover runTUI success and runner error branches

- **cli:** Harden service start-stop error branches

- **cli:** Close remaining history parser edge branches

- **cli:** Close banner and taxonomy layout edge branches

- **cli:** Close context model remaining edge branches

- **cli:** Harden files and logs edge branches

- **cli:** Harden palette fallback and toast view branches

- **cli:** Close remaining ui edge branches to 100 coverage

- **cli:** Remove titled-box assumptions and harden smoke loops

- **cli:** Add api command read and write matrix coverage

- **cli:** Expand non-interactive command edge coverage

- **cli:** Raise command-suite coverage with auth error matrix

- **cli:** Cover api command server-error matrix

- **cli:** Add validation matrix for api command inputs

- **cli:** Close remaining branches for full coverage

- Remove fake coverage-padding tests

- Remove fake coverage-padding tests


### Merge

- Metadata approval fixes into main

- Metadata executor normalization fix

- Bring metadata scope visibility fix

- Bring install/cicd parking changes into main


---
*Generated by [git-cliff](https://git-cliff.org)*
