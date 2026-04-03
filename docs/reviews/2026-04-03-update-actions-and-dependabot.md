# Review: update actions and dependabot

## Summary

I reviewed all files changed on `feature/update-actions-and-dependabot` versus `main`:

- `.github/dependabot.yml`
- `.github/workflows/ci-cd.yml`
- `.github/workflows/release.yml`
- `docs/plans/2026-04-03-update-actions-and-dependabot.md`

The GitHub Actions version bumps look consistent across both workflow files, all referenced tags exist upstream, and there are no missed action updates in other workflow files.

I found one substantive implementation issue and one matching documentation/spec issue.

---

## Issue 1 — The Dependabot cron expressions are not actually biweekly

**File:** `.github/dependabot.yml`

**Line context:**

- Lines 5-8
  ```yaml
  schedule:
    interval: "cron"
    cronjob: "0 6 */14 * *"
    timezone: "UTC"
  ```
- Lines 17-20
  ```yaml
  schedule:
    interval: "cron"
    cronjob: "0 7 8-31/14 * *"
    timezone: "UTC"
  ```

**Why this is a problem:**

These expressions step through the **day-of-month** field, not a true anchored 14-day interval.

- `*/14` in day-of-month expands within each month and resets at the month boundary.
- `8-31/14` likewise produces fixed calendar days within each month, not every 14 days across time.

That means the config does **not** satisfy the stated "every 2 weeks / biweekly" requirement. It can also produce uneven gaps around month boundaries.

Examples:

- `0 6 */14 * *` runs on days `1, 15, 29` of each month, then resets to `1` on the next month. So a run on the 29th can be followed by another on the 1st, only a couple of days later.
- `0 7 8-31/14 * *` runs on `8` and `22` of each month, which is semimonthly, not biweekly.

**Impact:**

Dependabot timing will be materially different from the plan and acceptance criteria. Reviewers/operators expecting a two-week cadence will instead get uneven monthly schedules.

**Fix instructions:**

Pick one of these approaches and update the config accordingly:

1. **If true biweekly cadence is required:** do not claim cron day-of-month stepping is biweekly. Replace this with a schedule that the team explicitly accepts as an approximation, or manage cadence outside Dependabot's native scheduler.
2. **If the real goal is low-noise grouped updates:** switch to a supported simple cadence such as `weekly` (with grouping and `open-pull-requests-limit: 1` kept as-is) and stagger weekdays/times between ecosystems.
3. **If semimonthly is acceptable:** keep cron, but document it accurately as semimonthly / specific calendar days rather than biweekly.

At minimum, the current implementation should not be merged while still described as biweekly.

---

## Issue 2 — The plan document states a biweekly behavior the implementation does not provide

**File:** `docs/plans/2026-04-03-update-actions-and-dependabot.md`

**Line context:**

- Lines 39-46
  ```md
  - schedule: every 2 weeks via cron
  ...
  - schedule: every 2 weeks via cron
  ```
- Lines 58-70
  ```yaml
  cronjob: "0 6 */14 * *"
  ...
  cronjob: "0 7 8-31/14 * *"
  ```
- Line 79
  ```md
  This keeps one grouped PR per ecosystem and staggers the two ecosystems so they do not open on the same run.
  ```
- Line 93
  ```md
  - Dependabot runs on a biweekly cadence per ecosystem.
  ```

**Why this is a problem:**

The plan and acceptance criteria encode the same incorrect assumption as the implementation: these cron expressions do not yield a true biweekly schedule.

If this lands as written, the repo documentation will misstate the actual behavior and may cause confusion later when Dependabot opens PRs on 1st/15th/29th or 8th/22nd style schedules instead of every 14 days.

**Fix instructions:**

Update the plan before marking it complete:

- either revise the requirement from **biweekly** to the actual supported cadence being used,
- or change the proposed implementation to a cadence that matches the written requirement.

Also adjust the acceptance criteria so they validate the real behavior, not the intended-but-unmet one.
