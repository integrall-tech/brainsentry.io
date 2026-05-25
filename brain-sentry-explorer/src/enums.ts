// Domain enums mirrored from brain-sentry-go/internal/domain/enums.go.
// Kept here so the example app is self-contained and the values it sends
// are guaranteed to match what the backend accepts.

export const CATEGORIES = [
  "INSIGHT",
  "DECISION",
  "WARNING",
  "KNOWLEDGE",
  "ACTION",
  "CONTEXT",
  "REFERENCE",
  "PATTERN",
  "ANTIPATTERN",
  "DOMAIN",
  "BUG",
  "OPTIMIZATION",
  "INTEGRATION",
] as const;
export type Category = (typeof CATEGORIES)[number];

export const IMPORTANCE_LEVELS = ["CRITICAL", "IMPORTANT", "MINOR"] as const;
export type Importance = (typeof IMPORTANCE_LEVELS)[number];

export const MEMORY_TYPES = [
  "SEMANTIC",
  "EPISODIC",
  "PROCEDURAL",
  "ASSOCIATIVE",
  "PERSONALITY",
  "PREFERENCE",
  "THREAD",
  "TASK",
  "EMOTION",
] as const;
export type MemoryType = (typeof MEMORY_TYPES)[number];

export const RELATIONSHIP_TYPES = [
  "USED_WITH",
  "CONFLICTS_WITH",
  "SUPERSEDES",
  "RELATED_TO",
  "REQUIRES",
  "PART_OF",
] as const;
export type RelationshipType = (typeof RELATIONSHIP_TYPES)[number];
