# System of Record

> Where the truth lives, what is derivable, and how to rebuild from
> scratch.

This document is the **contract** between developers and operators about
data ownership in brainsentry.io. Violating it (e.g. storing user-edited
content in a "cache" tier with no reconciler back to canonical) is a
correctness bug, not a performance choice.

---

## Canonical vs. Derived

### Canonical (system of record)

These are the only data stores that **own** their state. Losing them means
losing user/operator data permanently. They live in PostgreSQL.

| Table                          | What it owns                                |
|--------------------------------|---------------------------------------------|
| `memories`                     | every user-created or imported memory       |
| `memory_tags`                  | user-applied tags on memories               |
| `memory_relationships`         | manually-curated edges between memories     |
| `memory_versions`              | per-memory version history                  |
| `memory_version_tags`          | tag history snapshots                       |
| `users`                        | accounts                                    |
| `user_roles`                   | role assignments                            |
| `tenants`                      | tenant boundaries                           |
| `audit_logs`                   | immutable history of operator actions       |
| `audit_memories_created`       | audit detail: which memories were created   |
| `audit_memories_modified`      | audit detail: which memories were modified  |
| `audit_memories_accessed`      | audit detail: which memories were accessed  |
| `notes`                        | operator notes                              |
| `note_keywords`                | keywords on notes                           |
| `note_related_memories`        | curated note→memory links                   |
| `note_related_notes`           | curated note→note links                     |
| `hindsight_notes`              | hindsight (post-incident) notes             |
| `hindsight_related_memories`   | curated hindsight→memory links              |
| `hindsight_related_notes`      | curated hindsight→note links                |
| `hindsight_tags`               | tags on hindsight notes                     |
| `policies`                     | governance rules                            |
| `events`                       | timeline of decisions                       |
| `decisions`                    | architectural / product decisions           |
| `sessions`                     | conversation state                          |
| `session_observations`         | observations recorded inside sessions       |
| `webhooks`                     | webhook subscriptions                       |
| `webhook_events`               | webhook event records                       |
| `webhook_deliveries`           | webhook delivery attempts                   |
| `eval_candidates`              | (capture on) baseline retrieval set         |

**Backups must include all of the above and only the above.**

### Derived (rebuildable cache)

These exist to make queries fast. They can be deleted and reconstructed
from canonical sources at any time without losing user data. The
`brainsentry rebuild` command (see below) does exactly that.

| Store / table             | Derived from                                  | Rebuilder                  |
|---------------------------|-----------------------------------------------|----------------------------|
| FalkorDB graph nodes      | `memories.id`                                  | `rebuild --graph`          |
| FalkorDB graph edges      | `relationships`, plus auto-extracted edges     | `rebuild --graph`          |
| pgvector embeddings       | `memories.content`                             | `rebuild --embeddings`     |
| Louvain `communities`     | FalkorDB graph + recall scoring                | `rebuild --communities`    |
| Redis embedding cache     | embedding API responses                        | self-heals on first miss   |
| Redis ratelimit counters  | request stream                                 | self-heals on next request |
| `compressed_summaries`    | LLM compression of `memories.content`          | `rebuild --compress`       |
| Cross-session reflections | LLM consolidation across sessions              | `rebuild --reflect`        |

**Rule of thumb:** if a column is the result of an LLM call, an embedding
call, or a graph algorithm — it is derived. If a human typed it or an
agent recorded it, it is canonical.

---

## The Rebuild Contract

`brainsentry rebuild --from <source> [--confirm-destructive]` reconstructs
every derived store from canonical Postgres. It is **idempotent** — running
it twice produces byte-identical artifacts.

```
brainsentry rebuild --from postgres --confirm-destructive
  ├─ TRUNCATE communities, compressed_summaries, ...
  ├─ Drop FalkorDB graph
  ├─ For every memory:
  │    ├─ re-embed (calls embedding provider)
  │    ├─ insert graph node
  │    └─ insert manually-curated edges
  ├─ Run Louvain → repopulate communities
  └─ Run compression → repopulate compressed_summaries
```

Disaster recovery is therefore: restore the Postgres backup, run rebuild.
Nothing else. **No backup of FalkorDB or Redis is part of the
recovery plan.** Backing them up is a waste of the operator's time and
gives a false sense of safety.

---

## CI Gate: `scripts/check-system-of-record.sh`

The script greps `internal/repository/postgres/migrations/*.up.sql` for
`CREATE TABLE` statements and refuses to merge if a new table is added
that does **not** appear in either:

- the canonical table list above (this file's `## Canonical` section), OR
- a comment in the migration that begins with `-- DERIVED:` followed by
  the rebuild flag that owns it (e.g. `-- DERIVED: rebuild --communities`).

This forces every new table to declare its tier at PR time, so the gap
between docs and reality stays small.

---

## When you add a new table

1. Decide: canonical or derived?
2. If canonical, add it to the table above and add it to the disaster-
   recovery runbook.
3. If derived, add a `-- DERIVED: rebuild --<flag>` comment at the top of
   the migration AND extend `brainsentry rebuild` to repopulate it.
4. Run `scripts/check-system-of-record.sh` locally before opening the PR.

---

## When in doubt

A column is canonical if losing it would force the user to re-do work
(re-type a memory, re-make a decision, re-confirm a policy). It is
derived if losing it would only force the system to spend CPU / LLM tokens
to recompute. **If you're not sure, treat it as canonical** — over-
classification is a small backup cost; under-classification is silent
data loss.
