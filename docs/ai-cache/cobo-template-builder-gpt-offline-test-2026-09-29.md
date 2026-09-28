# CoBo CMS Template Builder — offline test

Date: 2026-09-29  
Scope: private Custom GPT only; no CMS API Action is configured.

## Result

The test attachment was accepted and evaluated by the GPT. The original candidate failed the GPT's Knowledge-based self-check; no backend validation was claimed or performed.

The GPT correctly identified these material issues:

- non-JSON wrapper text around the object;
- `template.periodicity = "annually"` (must be `"yearly"` for a periodic template);
- `template.deadline_config.frequency_unit = "ANNUALLY"` (must be `"YEARLY"`);
- unsupported company-class value `"public"`;
- a descriptive, multi-value `template.format` instead of one file-type token (candidate uses `"PDF"`);
- `NEXT_SLOT` with an unnecessary applicable slot; and
- normalization of the configured deadline to `T+110`.

It produced `bao-cao-thuong-nien.candidate.json`, stated that it is a candidate only, and instructed the user to resolve the three department mappings in CMS before validating there. It explicitly stated that no CMS template was created.

## Acceptance status

`GPT_OFFLINE_TEST=PASS`

`BACKEND_VALIDATION=NOT_RUN`

`CMS_ACTIONS=NOT_CONFIGURED`

`CMS_DRAFT_CREATED=false`

`PRODUCTION_TOUCHED=false`

## Limitation

This proves the private GPT's Knowledge-guided transformation and disclosure behavior only. The generated candidate must still be uploaded to CMS and pass the live validate endpoint before any confirm action.

## DEV validate follow-up

The user supplied the DEV validate result for the generated candidate. It reported `parse_valid=true`, `domain_valid=true`, `activation_ready=true`, and no errors. The only blockers were unresolved department mappings:

- `legal` / Phòng Pháp chế & Tuân thủ → `dept-001` (Phòng Pháp chế);
- `bod` / Ban Giám đốc → `dept-004` (Ban Tổng Giám đốc).

`corporate_secretary` was auto-matched. `can_confirm` remained false until the two mappings are supplied. No confirm action was requested or performed. Authentication material from the pasted response is intentionally not recorded.
