// API DTOs and zod schemas for the brainsentry.io memory surface.
//
// The schemas are intentionally LOOSE (.passthrough(), most fields
// optional). They exist to assert the *shape* of a response — the fields
// the example app depends on — not to reject every extra field. The
// validation scenarios use them to catch regressions like a missing id
// or a renamed field.

import { z } from "zod";
import type { Category, Importance, MemoryType } from "../enums.js";

// --- Auth ---

export const loginResponseSchema = z
  .object({
    token: z.string().min(1),
    refreshToken: z.string().optional(),
    tenantId: z.string().optional(),
    user: z
      .object({
        id: z.string(),
        email: z.string().optional(),
        roles: z.array(z.string()).optional(),
      })
      .passthrough()
      .optional(),
  })
  .passthrough();
export type LoginResponse = z.infer<typeof loginResponseSchema>;

// --- Memory ---

// Create returns domain.Memory; Get returns dto.MemoryResponse. The two
// shapes overlap but are not identical — this schema covers the common
// fields the app relies on, so it validates against both.
export const memorySchema = z
  .object({
    id: z.string().min(1),
    content: z.string(),
    summary: z.string().optional(),
    category: z.string().optional(),
    importance: z.string().optional(),
    memoryType: z.string().optional(),
    tags: z.array(z.string()).optional().nullable(),
    version: z.number().optional(),
    helpfulCount: z.number().optional(),
    notHelpfulCount: z.number().optional(),
    accessCount: z.number().optional(),
    createdAt: z.string().optional(),
    updatedAt: z.string().optional(),
    tenantId: z.string().optional(),
  })
  .passthrough();
export type Memory = z.infer<typeof memorySchema>;

export const memoryListSchema = z
  .object({
    memories: z.array(memorySchema),
    page: z.number(),
    size: z.number(),
    totalElements: z.number(),
    totalPages: z.number(),
    hasNext: z.boolean(),
    hasPrevious: z.boolean(),
  })
  .passthrough();
export type MemoryList = z.infer<typeof memoryListSchema>;

export const searchResponseSchema = z
  .object({
    results: z.array(memorySchema),
    total: z.number(),
    searchTimeMs: z.number().optional(),
  })
  .passthrough();
export type SearchResponse = z.infer<typeof searchResponseSchema>;

export const feedbackWeightSchema = z
  .object({
    memoryId: z.string(),
    helpfulCount: z.number(),
    notHelpfulCount: z.number(),
    feedbackWeight: z.number(),
  })
  .passthrough();

export const relationshipSchema = z
  .object({
    id: z.string().optional(),
    fromMemoryId: z.string().optional(),
    toMemoryId: z.string().optional(),
    type: z.string().optional(),
    strength: z.number().optional(),
  })
  .passthrough();

// --- Request shapes ---

export interface CreateMemoryRequest {
  content: string;
  summary?: string;
  category?: Category;
  importance?: Importance;
  memoryType?: MemoryType;
  tags?: string[];
  metadata?: Record<string, unknown>;
}

export interface UpdateMemoryRequest {
  content?: string;
  summary?: string;
  category?: Category;
  importance?: Importance;
  tags?: string[];
  changeReason?: string;
}

export interface SearchRequest {
  query: string;
  categories?: Category[];
  minImportance?: Importance;
  tags?: string[];
  limit?: number;
  includeRelated?: boolean;
}
