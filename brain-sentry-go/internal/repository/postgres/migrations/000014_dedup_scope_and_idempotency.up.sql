-- Dedup escopado + idempotência por origem (correções pré-Fatia B do VendaX).
--
-- CONTEXTO. Os fatos do VendaX são templates preenchidos por evento, um por
-- cliente, e o texto de dois clientes diferentes é frequentemente IDÊNTICO:
--
--   "recusou SKU-9182: não trabalha com a marca"  tags: [cliente:acme-001]
--   "recusou SKU-9182: não trabalha com a marca"  tags: [cliente:beta-002]
--
-- O dedup por SimHash comparava contra o tenant inteiro, então o segundo POST
-- caía em distância 0, não criava nada e devolvia o id de uma memória do
-- OUTRO cliente. Falha silenciosa no eixo mais sensível da integração.

-- ---------------------------------------------------------------------------
-- 1. Idempotência: uma origem, uma memória.
-- ---------------------------------------------------------------------------
-- O outbox do Core é at-least-once: retenta quando não tem certeza de que a
-- chamada chegou. Antes, quem absorvia a retentativa era o dedup por conteúdo
-- — que esta mudança (corretamente) restringe. Sem esta restrição, a
-- retentativa passaria a criar fato duplicado.
--
-- Índice PARCIAL porque source_reference é opcional: memórias sem origem (a
-- maioria das criadas por humano) não competem entre si por NULL/''.
--
-- Verificado em produção antes de criar: zero grupos duplicados
-- (SELECT ... GROUP BY tenant_id, source_reference HAVING count(*) > 1 → 0),
-- então não há dado pré-existente a reconciliar.
CREATE UNIQUE INDEX IF NOT EXISTS idx_memories_source_reference_unique
    ON memories (tenant_id, source_reference)
    WHERE source_reference IS NOT NULL
      AND source_reference <> ''
      AND deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- 2. Dedup deixa de varrer a tabela: pré-filtro por blocos do SimHash.
-- ---------------------------------------------------------------------------
-- A consulta antiga carregava TODOS os hashes do tenant num mapa a cada
-- create e comparava linearmente — O(n) por escrita, num caminho que a Fatia B
-- percorre em lote.
--
-- Princípio da casa dos pombos: o SimHash tem 64 bits (16 chars hex) e o
-- limiar é Hamming <= 3. Partindo em 4 blocos de 16 bits, três bits diferentes
-- não conseguem tocar os quatro blocos — pelo menos um bloco é IDÊNTICO. Logo
-- "algum bloco bate" é um pré-filtro sem perda: nunca descarta um candidato
-- que a comparação completa aceitaria.
CREATE INDEX IF NOT EXISTS idx_memories_simhash_b0
    ON memories (tenant_id, substr(sim_hash, 1, 4))
    WHERE sim_hash <> '' AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_memories_simhash_b1
    ON memories (tenant_id, substr(sim_hash, 5, 4))
    WHERE sim_hash <> '' AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_memories_simhash_b2
    ON memories (tenant_id, substr(sim_hash, 9, 4))
    WHERE sim_hash <> '' AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_memories_simhash_b3
    ON memories (tenant_id, substr(sim_hash, 13, 4))
    WHERE sim_hash <> '' AND deleted_at IS NULL;

-- ---------------------------------------------------------------------------
-- 3. Escopo por tag na consulta de dedup.
-- ---------------------------------------------------------------------------
-- memory_tags só tinha a PK (memory_id, tag), que não serve para "quais
-- memórias têm ESTA tag" — o lado pelo qual o dedup agora pergunta.
CREATE INDEX IF NOT EXISTS idx_memory_tags_tag
    ON memory_tags (tag, memory_id);
