// Realistic sales-CRM seed corpus — 20 multi-paragraph memories spanning
// 4 customers (Acme, Globex, Initech, Hooli), 2 salespeople (Ana, Bruno)
// and 2 products (brain-sentry Pro/Enterprise). Plus 6 cross-cutting
// patterns/insights/decisions/bugs to give filter and category scenarios
// something to chew on.
//
// Each seed has a stable `key`. Scenarios reference seeds by key; the
// seeding helper returns a Map<key, memoryId> so assertions can name the
// expected-relevant memories instead of UUIDs.

import { createMemory } from "../api/memories.js";
import type { BrainSentryClient } from "../api/client.js";
import type { Category, Importance, MemoryType } from "../enums.js";
import { memorySchema } from "../api/types.js";
import { expectShape } from "./assert.js";

export interface SalesSeed {
  key: string;
  content: string;
  summary: string;
  category: Category;
  importance: Importance;
  memoryType?: MemoryType;
  tags: string[];
  metadata: Record<string, unknown>;
}

export const SALES_CORPUS: SalesSeed[] = [
  // ────── ACME CORP (data-science team, evaluating Pro) ──────
  {
    key: "acme-discovery",
    content:
      "Discovery call with Acme Corp on 2026-04-12. Ana led the call with CTO Maria Santos and her head of data Lucas Ferreira. Acme's data-science team has 12 engineers building credit-risk models on Postgres 15 — they have NOT migrated to 16+ and have no concrete timeline to do so. Maria flagged this as a hard blocker if brain-sentry requires pgvector on PG16. Budget is described as 'tight' for FY2026; a decision is expected by end of Q3 (September 2026). Maria is the technical decision-maker; her CFO Roberto Lima holds the budget.",
    summary: "Acme discovery — 12-person DS team, Postgres 15 blocker, Q3 decision, Maria Santos is DM.",
    category: "CONTEXT",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:acme", "stage:discovery", "owner:ana", "product:pro"],
    metadata: { customerId: "acme", salesperson: "ana", stage: "discovery", date: "2026-04-12" },
  },
  {
    key: "acme-demo",
    content:
      "Live product demo with Acme on 2026-04-18. Ana presented; Maria Santos plus two senior engineers (Lucas Ferreira and Camila Reis) attended. Reception was warm — Camila explicitly called out the SimHash dedup as 'exactly what we need for our agent memory consolidation'. The recurring concern was the pgvector hard requirement clashing with their Postgres 15 environment; Lucas asked twice whether a fallback exists. Ana committed to confirming whether brain-sentry can run with float4[] vectors instead of pgvector for the memory core, and to providing a written upgrade impact estimate.",
    summary: "Acme demo positive on dedup; pgvector vs PG15 still the main objection.",
    category: "CONTEXT",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:acme", "stage:demo", "owner:ana", "product:pro"],
    metadata: { customerId: "acme", salesperson: "ana", stage: "demo", date: "2026-04-18" },
  },
  {
    key: "acme-objection-sso",
    content:
      "Acme objection raised on 2026-04-22 in an email thread initiated by Maria Santos: their security policy mandates SSO via Okta for any third-party SaaS handling production data. Brain-sentry Pro does not include SSO; only the Enterprise tier ships the Okta + SAML integration. Bruno escalated to Ana, who proposed upgrading the proposal to Enterprise (or a Pro+SSO add-on if pricing permits). Maria has not yet replied to the upgrade suggestion; ETA is end of the week.",
    summary: "Acme blocker: Okta SSO is Enterprise-only. Upgrade proposal pending response.",
    category: "WARNING",
    importance: "CRITICAL",
    tags: ["bs-explorer", "customer:acme", "stage:objection", "owner:ana", "feature:sso", "feature:okta"],
    metadata: { customerId: "acme", salesperson: "ana", stage: "objection", date: "2026-04-22" },
  },
  {
    key: "acme-followup-plan",
    content:
      "Next action for Acme, committed on the 04-18 demo and reaffirmed on 04-22: Ana to send a POC plan by April 30 covering (1) data ingestion path from their existing pipelines, (2) clarification on the pgvector vs float4[] storage option, (3) updated quote with Enterprise tier including Okta SSO. Maria expects to circulate internally and respond by mid-May. If no response by 2026-05-20, Bruno is to ping with a soft nudge — not a discount offer.",
    summary: "Acme next action: POC plan + Enterprise quote by April 30; nudge after May 20.",
    category: "ACTION",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:acme", "stage:followup", "owner:ana"],
    metadata: { customerId: "acme", salesperson: "ana", stage: "followup", dueDate: "2026-04-30" },
  },

  // ────── GLOBEX INDUSTRIES (CLOSED-WON, Enterprise) ──────
  {
    key: "globex-demo",
    content:
      "Initial demo with Globex Industries on 2026-03-05. Bruno qualified inbound; demo led by Ana with CISO Carlos Mendes and his deputy Isabela Costa. Globex is a regulated fintech; their primary interest in brain-sentry was the audit log + provenance export (W3C PROV-O) for compliance reasons. Carlos asked about row-level tenant isolation depth — Ana walked through the middleware Tenant resolution and the `tenant_id` filter on every query. He was satisfied.",
    summary: "Globex demo — fintech, audit-log + PROV-O compliance angle. CISO Carlos Mendes is DM.",
    category: "CONTEXT",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:globex", "stage:demo", "owner:ana", "product:enterprise", "vertical:fintech"],
    metadata: { customerId: "globex", salesperson: "ana", stage: "demo", date: "2026-03-05" },
  },
  {
    key: "globex-negotiation",
    content:
      "Pricing negotiation with Globex on 2026-03-18. Ana presented the standard Enterprise pricing ($96k/year for the agreed seat count). Carlos pushed for a 20% discount citing budget constraints and the 3-year intent. Ana countered with 15% on year 1, returning to list on year 2 and year 3, plus prepayment incentive. Roberto from procurement signed off on Ana's counter; deal value: $80k year 1, $96k years 2 and 3.",
    summary: "Globex pricing: 15% discount year 1, list year 2-3. $80k → $96k → $96k. Approved.",
    category: "CONTEXT",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:globex", "stage:negotiation", "owner:ana", "product:enterprise"],
    metadata: { customerId: "globex", salesperson: "ana", stage: "negotiation", date: "2026-03-18" },
  },
  {
    key: "globex-decision-contract",
    content:
      "Decision: Globex Industries signed the brain-sentry Enterprise contract on 2026-03-25. Three-year commit, $80k year 1 (15% discount approved), $96k years 2 and 3. Total contract value $272k. Decision rationale: compliance (PROV-O audit) and tenant isolation depth outweighed competitor pricing. Decided by CISO Carlos Mendes with procurement (Roberto) and CFO (Daniel Rocha) sign-off. This is our first regulated-fintech Enterprise win and sets a reference for the vertical.",
    summary: "Globex SIGNED Enterprise — $272k 3y. First fintech-regulated reference.",
    category: "DECISION",
    importance: "CRITICAL",
    tags: ["bs-explorer", "customer:globex", "stage:closed-won", "owner:ana", "product:enterprise", "vertical:fintech", "outcome:win"],
    metadata: { customerId: "globex", salesperson: "ana", stage: "closed-won", date: "2026-03-25", contractValueUSD: 272000 },
  },
  {
    key: "globex-onboarding",
    content:
      "Globex onboarding week 1 (2026-04-01 to 2026-04-07). Activation by their AI platform team led by Isabela Costa was rapid — 3,047 memories created in the first 7 days from their internal agent traffic. Most popular categories: KNOWLEDGE (61%), DECISION (18%), CONTEXT (14%). Carlos asked about scaling pgvector beyond 100k embeddings; Ana looped in Solutions Eng to schedule a tuning call. Health signals strong: this account is a candidate for case study and reference.",
    summary: "Globex onboarding healthy — 3k memories week 1, pgvector scaling question pending.",
    category: "INSIGHT",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:globex", "stage:onboarding", "owner:ana", "product:enterprise", "health:green"],
    metadata: { customerId: "globex", salesperson: "ana", stage: "onboarding", date: "2026-04-07" },
  },

  // ────── INITECH (CLOSED-LOST, lessons learned) ──────
  {
    key: "initech-discovery",
    content:
      "Discovery call with Initech on 2026-02-14. Bruno qualified; call led by Ana with their VP Eng Tony Park and architect Lin Chen. Initech is mid-stage SaaS rebuilding their agent platform. They were transparent that they were also evaluating MemCo and BrainBridge in parallel. Tony emphasized price as the primary lever and said the technical bar was 'good enough' across all three; differentiation would come on cost. Decision target was end of February.",
    summary: "Initech discovery — competing with MemCo + BrainBridge, price-led decision.",
    category: "CONTEXT",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:initech", "stage:discovery", "owner:ana", "competitor:memco", "competitor:brainbridge"],
    metadata: { customerId: "initech", salesperson: "ana", stage: "discovery", date: "2026-02-14" },
  },
  {
    key: "initech-proposal",
    content:
      "Proposal sent to Initech on 2026-02-20. Brain-sentry Pro at $36k/year for the agreed seat count, with an introductory 10% first-year discount bringing it to $32.4k. Ana included a comparative one-pager highlighting the dedup, audit, and bi-temporal features. Tony acknowledged receipt and asked whether the price was final; Ana confirmed it was the best offer.",
    summary: "Initech proposal: $32.4k year 1 (10% discount). Pricing presented as final.",
    category: "CONTEXT",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:initech", "stage:proposal", "owner:ana", "product:pro"],
    metadata: { customerId: "initech", salesperson: "ana", stage: "proposal", date: "2026-02-20" },
  },
  {
    key: "initech-decision-loss",
    content:
      "Decision: Initech selected MemCo on 2026-02-28. MemCo's offer was $24k year 1 (33% lower than ours). Tony was candid in the loss email: 'features are comparable, price isn't'. Lessons captured: (1) when prospect openly says 'price-led', match aggressively in proposal 1 instead of presenting list — we left margin on the table for no reason. (2) BrainBridge was apparently a stalking horse; MemCo was the real competitor from week 1. (3) Tony would consider us at renewal if MemCo disappoints.",
    summary: "Initech LOST to MemCo on price ($24k vs $32k). Lessons: read 'price-led' signals earlier.",
    category: "DECISION",
    importance: "CRITICAL",
    tags: ["bs-explorer", "customer:initech", "stage:closed-lost", "owner:ana", "competitor:memco", "outcome:loss"],
    metadata: { customerId: "initech", salesperson: "ana", stage: "closed-lost", date: "2026-02-28", competitor: "memco" },
  },

  // ────── HOOLI (DORMANT, follow-up failure) ──────
  {
    key: "hooli-discovery",
    content:
      "Discovery call with Hooli on 2026-03-10. Bruno led intake; conversation with VP Engineering Patricia Wilson was promising. Patricia is rebuilding their internal copilot infrastructure and said brain-sentry's bi-temporal model was 'exactly the abstraction we were going to build ourselves'. Strong technical fit. Patricia committed to a deeper dive demo within 2 weeks and to pulling in her CISO for the security conversation.",
    summary: "Hooli discovery — strong fit, Patricia Wilson is champion, deeper demo committed in 2 weeks.",
    category: "CONTEXT",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:hooli", "stage:discovery", "owner:bruno"],
    metadata: { customerId: "hooli", salesperson: "bruno", stage: "discovery", date: "2026-03-10" },
  },
  {
    key: "hooli-demo-cancelled",
    content:
      "Demo with Hooli scheduled for 2026-03-24 was cancelled by Patricia 90 minutes before start, citing an internal 'all-hands incident'. Bruno proposed three new slots that week; Patricia accepted 2026-03-28 but cancelled that one too, this time without a reason given. Bruno noted: 'last-minute cancellations twice in a row, low signal but worth flagging'.",
    summary: "Hooli demo cancelled twice last-minute. Yellow flag on engagement.",
    category: "WARNING",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:hooli", "stage:demo", "owner:bruno", "health:yellow"],
    metadata: { customerId: "hooli", salesperson: "bruno", stage: "demo-cancelled", date: "2026-03-28" },
  },
  {
    key: "hooli-ghosted",
    content:
      "Hooli has now ignored three follow-up emails from Bruno (2026-04-02, 2026-04-09, 2026-04-16). Patricia's last reply was on 2026-03-24 (the cancellation). LinkedIn shows no public change in her role. Action: mark Hooli as dormant in the CRM; move out of active pipeline. Re-attempt in Q3 with a different angle (case study from a peer in their vertical, e.g. Globex once we publish their reference).",
    summary: "Hooli marked DORMANT after 3 ignored follow-ups. Re-attempt Q3 with Globex case study.",
    category: "ACTION",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "customer:hooli", "stage:dormant", "owner:bruno", "health:red"],
    metadata: { customerId: "hooli", salesperson: "bruno", stage: "dormant", date: "2026-04-16" },
  },

  // ────── CROSS-CUTTING patterns / insights / org decisions ──────
  {
    key: "pattern-edu-discount",
    content:
      "Pattern observed across multiple deals: prospects from the education vertical (universities, edtech startups) consistently request an educational discount even when not in the standard discount matrix. Of the last 8 EDU prospects, 6 explicitly asked. Recommended response: pre-empt with a 25% educational tier offer in proposal 1 — closes faster (median 11 days vs 23 in the data we have).",
    summary: "PATTERN: EDU prospects always ask for discount. Offer 25% upfront — closes 2x faster.",
    category: "PATTERN",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "pattern:discount", "vertical:education", "owner:ana"],
    metadata: { pattern: "edu-discount", sampleSize: 8 },
  },
  {
    key: "antipattern-friday-emails",
    content:
      "Antipattern: outbound emails sent on Fridays after 14:00 BRT get a 60% lower reply rate than emails sent Monday-Thursday morning. Sample of 200 outbound touches last quarter. Recommended: schedule Friday-afternoon-composed emails to send Monday 09:00 via the CRM scheduler. Do not send Friday-afternoon directly.",
    summary: "ANTIPATTERN: Friday afternoon emails get 60% less reply. Schedule for Mon morning.",
    category: "ANTIPATTERN",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "antipattern:friday-emails", "owner:bruno"],
    metadata: { sampleSize: 200, replyRateDelta: -0.6 },
  },
  {
    key: "insight-2demo-close",
    content:
      "Insight from win/loss analysis Q1 2026: deals where the prospect attended TWO demos (typically discovery overview + technical deep-dive) closed at 80% rate (8 of 10). Deals with only one demo closed at 30% (3 of 10). Strong signal that the second demo correlates with internal champion-building. Recommendation: always offer the deep-dive even if prospect says 'one demo is enough'.",
    summary: "INSIGHT: 2-demo deals close at 80%, 1-demo at 30%. Always push for the deep-dive.",
    category: "INSIGHT",
    importance: "CRITICAL",
    tags: ["bs-explorer", "insight:2demo", "owner:ana"],
    metadata: { conversionTwoDemo: 0.8, conversionOneDemo: 0.3 },
  },
  {
    key: "decision-deprecate-starter",
    content:
      "Internal organizational decision (2026-04-05, exec team): the Starter tier ($12k/year) will be deprecated effective 2026-10-01. Reason: gross margin on Starter is negative once support and infra are allocated; the tier was originally a foot-in-the-door bet that did not produce the expected upgrade rate (only 8% of Starter customers upgrade within 12 months). Existing Starter customers grandfathered for 24 months; new Starter sales stop on 2026-10-01. Communicated to sales team on 2026-04-08.",
    summary: "DECISION (internal): Deprecate Starter tier 2026-10-01. Negative margin, 8% upgrade rate.",
    category: "DECISION",
    importance: "CRITICAL",
    tags: ["bs-explorer", "internal", "product:starter", "owner:exec"],
    metadata: { decisionDate: "2026-04-05", effectiveDate: "2026-10-01" },
  },
  {
    key: "bug-pricing-page",
    content:
      "Recurring bug reported by 4 different prospects in the last 6 weeks: the public pricing page presents Pro and Enterprise tiers in a layout where the 'SSO' row is ambiguous — it appears checked for both tiers but a footnote (rarely read) clarifies that Pro only supports Google Workspace SSO, not Okta or Azure AD. Prospects discover the limitation only in deeper conversations, sometimes after committing to Pro. Marketing has been notified; fix planned for 2026-05-15.",
    summary: "BUG: Pricing page SSO row ambiguous — Pro = Google only, Enterprise = Okta/Azure too.",
    category: "BUG",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "bug:pricing-page", "feature:sso", "owner:marketing"],
    metadata: { reportedBy: 4, fixDate: "2026-05-15" },
  },
  {
    key: "optimization-soc2-early",
    content:
      "Process optimization: sending the SOC2 Type II report PROACTIVELY in the first technical deep-dive (instead of waiting for security review request) accelerates the security-review phase by an average of 2 weeks. Sample of 6 deals where this was tried in Q1 2026; all 6 closed security review faster than the historical baseline. Adopted as standard practice for all Enterprise opportunities effective 2026-04-01.",
    summary: "OPTIMIZATION: Send SOC2 early in tech deep-dive — saves 2 weeks of security review.",
    category: "OPTIMIZATION",
    importance: "IMPORTANT",
    tags: ["bs-explorer", "optimization:soc2", "stage:security-review", "owner:ana"],
    metadata: { sampleSize: 6, timeSaved: "2 weeks" },
  },
];

/**
 * Delete any `[bs-explorer]`-prefixed memories left over from prior aborted
 * runs. Idempotent self-healing — keeps the suite re-runnable even when a
 * previous seed failed mid-way and skipped its cleanup. Failures are
 * swallowed (best-effort).
 */
export async function clearResidualBsxMemories(
  client: BrainSentryClient,
): Promise<number> {
  const call = await client.request<{
    memories?: { id: string; content?: string }[];
  }>("GET", "/v1/memories", { query: { page: 0, size: 200 } });
  if (!call.ok || !call.data?.memories) return 0;
  const orphans = call.data.memories.filter(
    (m) =>
      typeof m.content === "string" && m.content.startsWith("[bs-explorer]"),
  );
  for (const m of orphans) {
    await client.request("DELETE", `/v1/memories/${m.id}`);
  }
  return orphans.length;
}

/**
 * Create every seed in the corpus and return a map from key → memory id.
 *
 * Accepts an optional `outIds` map so callers can pre-set `vars.corpus = new Map()`
 * and have it populated progressively. If the seed fails mid-way (timeout,
 * 429 exhaustion), `vars.corpus` already holds the ids that DID land — so
 * the scenario's cleanup step can still delete them.
 *
 * Also performs a residual sweep first so a prior aborted run can't pollute
 * the recall/precision metrics with extra `[bs-explorer]` rows.
 */
export async function seedSalesCorpus(
  client: BrainSentryClient,
  outIds?: Map<string, string>,
): Promise<Map<string, string>> {
  await clearResidualBsxMemories(client);
  const ids = outIds ?? new Map<string, string>();
  for (const seed of SALES_CORPUS) {
    const call = await createMemory(client, {
      content: seed.content,
      summary: seed.summary,
      category: seed.category,
      importance: seed.importance,
      memoryType: seed.memoryType,
      tags: seed.tags,
      metadata: seed.metadata,
    });
    const m = expectShape(call, 201, memorySchema);
    ids.set(seed.key, m.id);
  }
  return ids;
}

/** Best-effort delete every seeded memory (ignores failures). */
export async function cleanupSalesCorpus(
  client: BrainSentryClient,
  ids: Map<string, string>,
): Promise<void> {
  for (const id of ids.values()) {
    await client.request("DELETE", `/v1/memories/${id}`);
  }
}

/** Resolve a list of seed keys to their memory ids using the seeded map. */
export function idsFor(map: Map<string, string>, ...keys: string[]): string[] {
  return keys.map((k) => {
    const id = map.get(k);
    if (!id) throw new Error(`sales-corpus: no id seeded for key "${k}"`);
    return id;
  });
}
