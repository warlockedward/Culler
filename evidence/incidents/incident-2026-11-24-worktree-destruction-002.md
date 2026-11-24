# Incident 2026-11-24 · /private/tmp worktree directory destruction (second event) · incident-002

Date 2026-11-24. Second destruction event of `/private/tmp/culler-worktrees/` in this project; the first is recorded in `incident-2026-11-24-worktree-destruction-001.md` (a09s1 worktree, overnight, attribution E4 unknown because `uptime` then showed 6 days = no reboot).

## 1. Facts (E1)
- Detection: at 17:42 the command `cd /private/tmp/culler-worktrees/dev` failed with "no such file or directory"; `ls /private/tmp/culler-worktrees/` = "No such file or directory"; `git worktree list` showed the dev worktree as **prunable**.
- `uptime` at detection = "17:42 up 3:10" ⇒ the machine booted at approximately **14:32 today**, i.e. between the last successful dev-worktree operation of this session (the AM v0.2.6 registration commits 14e7d1d / 6c9e808 and the `git worktree remove` of a09s4, both of which succeeded while the directory still existed) and the detection moment.
- AGENTS.md (Git section) documents this exact hazard: "macOS deletes `/private/tmp` at boot. Commit work to the branch before the session ends."

## 2. Attribution
**Boot-time `/private/tmp` wipe** — the mechanism is documented in the repository's own operating rules and the timing is proven by `uptime` (a reboot occurred today within the loss window). This differs from incident-001, whose window contained no reboot (uptime 6 days at that detection) and therefore stayed **E4 unknown**. No operation in this session deletes parent directories: the only worktree mutation run was `git worktree remove …/a09s4` (scoped to one subdirectory, completed successfully before the reboot).

## 3. Loss assessment
**Zero data loss.** Everything committed in this session was committed to `shz/dev` (object store in the main checkout's `.git`, which is not under `/private/tmp`):
- `59d141d` S4a CLI command-name face (37 files),
- `14e7d1d` AM v0.2.6 registration,
- `6c9e808` DEV-BASELINE S2/S4a completion notes.
Post-recovery verification: recovered dev worktree tree clean; `docs/contracts/ACCEPTANCE-MATRIX.md` bytes = `f742897cd5761cf47366465dfd88b998df3f11404de4110d4a50fa175788249c` = the registered v0.2.6 value; the untracked dual-face copy in the main checkout is byte-identical (cmp DUAL-EQUAL-POST-RECOVERY).
What was lost: nothing beyond the working-tree checkout itself (untracked scratch files under the worktrees directory, if any, are gone — none were expected; the a09s4 evidence previews had already been committed into `59d141d`).

## 4. Recovery (3 commands, all exit 0)
```
git worktree prune
mkdir -p /private/tmp/culler-worktrees
git worktree add /private/tmp/culler-worktrees/dev shz/dev   # HEAD = 6c9e808
```

## 5. Discipline consequence
The standing rule "commit work to the branch before the session ends" is confirmed as load-bearing: this event destroyed the checkout but not the work. Residual risk registered (not fixed, out of scope for this incident): any **uncommitted** work in a `/private/tmp` worktree at boot time is lost without warning — the project's answer is the existing rule (no backup mechanism added).
