# Integração VendaX ↔ Brain Sentry

Guia de uso do que a **RFC-014 Parte I** entregou. O público é quem vai escrever
as fatias A–D no `vendax.ai` (o Core), não quem mantém o Brain Sentry.

- **Base URL de produção:** `https://api.brainsentry.com.br/api`
- **Fonte da verdade do desenho:** `vendax-spec/RFC-014-memoria-de-conversa-brainsentry.md`
- **Este documento descreve só o que existe hoje**, verificado contra o código.
  Onde algo ainda não existe, está dito explicitamente.

---

## 1. Credencial: uma chave por tenant

O Core **não** usa login de usuário. Usa uma chave de serviço, emitida por um
admin, presa a **um** tenant.

### Emitir (uma vez por tenant, por um humano com JWT de ADMIN)

```bash
curl -X POST https://api.brainsentry.com.br/api/v1/tenants/{tenantId}/api-keys \
  -H "Authorization: Bearer <JWT-de-admin>" \
  -H 'Content-Type: application/json' \
  -d '{"name":"vendax-core-rioquality"}'
```

```json
{
  "id": "9f1c…",
  "tenantId": "…",
  "name": "vendax-core-rioquality",
  "keyPrefix": "bs_a1b2c3d4",
  "createdAt": "2026-07-27T20:00:00Z",
  "secret": "bs_XmK9…",
  "warning": "store this secret now — it is not recoverable"
}
```

> **`secret` aparece uma única vez.** Só o hash é guardado. Chave perdida se
> substitui, não se recupera. Guarde cifrada, junto das credenciais de ERP do
> tenant.

Opcional: `"expiresAt": "2027-01-01T00:00:00Z"` para chave com prazo.

### Usar

Qualquer um dos dois headers:

```
Authorization: Bearer bs_XmK9…
X-API-Key: bs_XmK9…
```

O JWT continua funcionando em paralelo — a chave só é reconhecida pelo prefixo
`bs_`, então nada do que existia mudou.

### Revogar

```bash
curl -X DELETE https://api.brainsentry.com.br/api/v1/api-keys/{keyId} \
  -H "Authorization: Bearer <JWT-de-admin>"
```

Imediato e idempotente. A linha vira lápide: a trilha de qual chave agiu
sobrevive à revogação.

### O que a chave garante — e o que não garante

**Garante:** a chave do tenant A **não alcança** o tenant B. Mandar
`X-Tenant-ID: B` ou `?tenant=B` devolve **403**, não um resultado silencioso do
tenant errado. Vale inclusive se a chave tivesse papel de admin — ela nunca
carrega ADMIN, e o `TenantExtractor` recusa antes de olhar papel.

**Não garante:** isolamento **entre clientes dentro do mesmo tenant**. Isso é
responsabilidade do Core, via tag `cliente:{ref}` obrigatória em toda escrita e
toda busca. A RFC §6.1 chama o teste desse isolamento de o mais importante da
integração — ele mora no Core, não aqui.

**Não pode:** chave de serviço não emite nem lista chaves (403). Uma chave que
cunha chaves cunharia uma para outro tenant.

---

## 2. Escrever um fato

```bash
POST /v1/memories
```

```json
{
  "content": "recusou SKU-9182: não trabalha com a marca",
  "category": "DECISION",
  "importance": "IMPORTANT",
  "tags": ["cliente:acme-001", "tipo:recusa", "vendedor:v-77"],
  "sourceReference": "decisao:8f2a-…",
  "metadata": {"sku": "9182", "motivo": "NAO_TRABALHA"},
  "validTo": "2026-10-25T00:00:00Z",
  "provenance": "EXPLICIT"
}
```

Campos que importam para esta integração:

| Campo | Por quê |
| --- | --- |
| `content` | o **fato derivado**, nunca a transcrição (RFC §3). Obrigatório, máx. 10000 |
| `tags` | **`cliente:{ref}` é obrigatória**. Sem ela o fato é irrecuperável no escopo certo — e pior, recuperável no escopo errado |
| `sourceReference` | o evento de domínio que gerou (`decisao:{id}`, `cotacao:{id}`, `resolucao:{id}`, `sentimento:{conv}`). É a chave da auditoria e do purge |
| `validTo` | prazo **por tipo de fato** (RFC §9.1), não um TTL global. Ver tabela abaixo |
| `metadata` | pares chave/valor filtráveis na busca exata |
| `category` | `DECISION`, `KNOWLEDGE`, `PATTERN`, `CONTEXT`, `INSIGHT`, `WARNING`, `ACTION`, `REFERENCE`, `ANTIPATTERN`, `DOMAIN`, `BUG` |
| `importance` | `CRITICAL`, `IMPORTANT`, `MINOR` — **só estes três**. O campo não é validado, então um valor fora da lista é aceito e depois não casa com nenhum filtro |
| `provenance` | `EXPLICIT` (decisão registrada pelo vendedor), `OBSERVED` (derivado de conversa), `INFERRED` (sentimento), `VALIDATED`, `CORRECTED`, `IMPORTED`. É o maior peso do trust score |

Prazos por tipo, conforme a RFC:

| Tipo de fato | `validTo` |
| --- | --- |
| recusa `JA_TEM_ESTOQUE` | `ciclo/2` do perfil de reposição |
| recusa `NAO_TRABALHA` / `MARCA_ERRADA` | ausente (preferência estrutural) |
| léxico de resolução | longo |
| sentimento | curto — é **estado**, não fato |
| cotação firmada | ausente (evento histórico) |

Resposta `201` com o objeto criado (`id`, `tenantId`, `createdAt`, …).

> **Sempre pelo outbox.** Gravar memória não pode derrubar uma cotação. O
> Brain Sentry não sabe nada sobre isso — a garantia é do Core (RFC §5.1).

---

## 3. Ler

### 3.1 Busca semântica — o caminho do `obter_cliente_360`

```json
POST /v1/memories/search
{
  "query": "preferências de marca",
  "tags": ["cliente:acme-001"],
  "limit": 10
}
```

A tag de cliente é o que mantém o escopo. **O Brain Sentry não a impõe** — se o
Core esquecer, virá memória de outro cliente do mesmo tenant.

Resposta:

```json
{"results": [ {"id":"…","content":"…","tags":["cliente:acme-001"],"createdAt":"…"} ],
 "total": 1, "searchTimeMs": 42}
```

Mapeamento para a `MemoriaClientePort` que já existe no Core:

| Brain Sentry | `Fato` |
| --- | --- |
| `content` | `fato` |
| trust score (`GET /v1/memories/{id}` → `trust`) | `confianca` |
| `validFrom` | `observadoEm` |

### 3.2 Busca exata — o caminho da auditoria

Quando `sourceReference` **ou** `metadata` vêm preenchidos, a busca vira
determinística: sem embedding, sem expansão de query, sem ranking por
similaridade, ordenada por `created_at desc`.

```json
POST /v1/memories/search
{"sourceReference": "decisao:8f2a-…"}
```

```json
POST /v1/memories/search
{"metadata": {"sku": "9182", "motivo": "NAO_TRABALHA"}}
```

`query` é **opcional** nesse modo. `metadata` casa por containment: os pares
informados precisam existir, chaves extras na memória não atrapalham.

Use isto — e não busca semântica — para "me devolva o fato gerado pelo evento
X". Ranquear uma chave conhecida por similaridade pode devolver uma memória
**diferente** que por acaso está perto no espaço vetorial, e a auditoria
compararia o fato errado com a fonte.

---

## 4. Revogar em lote (auditoria)

```json
POST /v1/memories/batch-expire
{"sourceReference": "decisao:8f2a-…", "reason": "decisão revertida no Core"}
```

ou por ids:

```json
{"ids": ["…","…"], "reason": "restrição removida"}
```

```json
{"expired": 3, "ids": ["…","…","…"]}
```

- **Não é delete.** Fecha a janela (`valid_to = now`) e grava o motivo em
  `metadata.expiredReason`. O fato ERA verdade — `/v1/memories/as-of` continua
  respondendo certo sobre o passado.
- `reason` é **obrigatório**: revogação em massa sem motivo é inauditável
  depois, o que anula o propósito de uma rotina cujo produto é um relatório.
- Uma chamada, uma transação. Um-a-um pode deixar o conjunto meio revogado se o
  chamador morrer no meio.

---

## 5. Retenção e remoção do titular

### 5.1 Política, por tenant

Vive em `tenants.settings`, não no código — o prazo é decisão jurídica e muda
por cliente:

```json
{"retention": {"purgeAfterValidToDays": 90, "maxPerRun": 500}}
```

`purgeAfterValidToDays` é a carência **entre o `validTo` e a remoção de fato**.
**Tenant sem política é no-op** — nunca um default que apaga. Errar para o lado
de reter é recuperável; o contrário não.

```bash
POST /v1/retention/run           # dry-run: diz o que faria
POST /v1/retention/run -d '{"confirm": true}'
```

### 5.2 Remoção do titular

```json
POST /v1/privacy/erasure
{"tag": "cliente:acme-001", "reason": "pedido de remoção — ticket #4412", "confirm": true}
```

Escopo por `tag`, `memoryIds` ou `sourceReference`. **Sem `confirm` é dry-run.**

```json
{
  "receiptId": "…", "kind": "erasure", "executed": true,
  "matchedMemories": 12,
  "counts": {"memories": 12, "memory_versions": 30, "memory_tags": 24, "…": 0},
  "auditRowsRedacted": 5,
  "graphPurged": true
}
```

O que a remoção cobre — e por que é mais que um DELETE:

- `memories` **e** `memory_versions` (que guarda o texto de **toda versão
  anterior**), `memory_relationships`, `memory_tags`, vínculos de nota e
  hindsight, e as três tabelas `audit_memories_*`;
- **os nós do FalkorDB**, que copiam `content` e `summary`;
- `audit_logs`: o **evento sobrevive** (quem, quando, qual id) e o conteúdo é
  anulado. Apagar a trilha destruiria a prova de que a própria remoção ocorreu.

Inclui memórias já soft-deletadas — o texto delas continuava na linha.

**Recibo.** Toda execução (inclusive dry-run e "não achei nada") gera recibo:

```bash
GET /v1/privacy/receipts
```

Identificadores, contagens e horário — **nunca** o conteúdo apagado. É o que
responde "isto foi removido?" depois que nada mais consegue responder.

### 5.3 O que ainda NÃO é coberto

- **Cache de embedding no Redis** não é invalidado. É derivado do texto, não o
  texto, e a chave é hash — mas se o requisito for rigor total, avise.
- **`decisions` e `events`** guardam `reasoning`/`description` próprios e não
  entram no escopo por `cliente:{ref}` (não têm a tag). **Se o Core passar a
  gravar texto do titular nessas superfícies na Fatia B, elas precisam entrar
  no purge** — amarre isso junto com a escrita.

---

## 6. Erros, latência e degradação

Erros vêm no formato:

```json
{"error":"Bad Request","message":"…","status":400,"errorCode":"validation","errorCategory":"VALIDATION"}
```

| Código | Quando | O que o Core faz |
| --- | --- | --- |
| `401` | chave inválida, revogada ou expirada | não retentar; alertar. É configuração, não intermitência |
| `403` | chave tentando outro tenant | **bug no Core** — a chave já define o tenant; não mande `X-Tenant-ID` |
| `400` | payload inválido (`content` vazio, `reason` ausente, escopo vazio) | não retentar |
| `5xx` / timeout | indisponibilidade | leitura degrada para `Optional.empty()`; escrita fica no outbox e retenta |

Latência: o Brain Sentry roda em **outra infra** (VPS `31.97.240.217`) que não a
do Core, então vale o desenho conservador da RFC §6.3 — timeout curto (~800 ms),
circuit breaker, e **recall fora do caminho quente da cotação**. Sem memória, o
prompt é o de hoje: o agente perde contexto, não a capacidade de trabalhar.

Escrita com LLM (compressão, extração) roda **assíncrona** no Brain Sentry — o
`POST /v1/memories` responde em ~1,4 s hoje e não espera por ela.

---

## 7. Armadilhas conhecidas

**O grafo não é escrito em tempo real.** `/v1/graph/*` (ego, GraphRAG,
comunidades) só reflete o que existia no último rebuild — o backend nunca
escreve no FalkorDB durante a operação. Há cron diário às 03:30. Se a Fatia C
depender de grafo vivo, isso precisa ser resolvido antes.

**`tenantId` no corpo é ignorado quando você usa chave de serviço.** O tenant
vem sempre da chave. Mandar outro não muda nada — e mandar no header dá 403.

**Não dê um segundo servidor MCP ao agente** (RFC §5.3). A allowlist da skill é
por servidor; um segundo servidor é uma segunda allowlist e um caminho para
contornar a primeira. Quem fala com o Brain Sentry é o Core.

**Não indexe mensagem a mensagem.** O que vira memória é o fato derivado. Uma
conversa de WhatsApp é majoritariamente coordenação ("ok", "bom dia"), e
indexá-la faz a recuperação devolver ruído com boa pontuação de similaridade.

---

## 8. Checklist antes de ligar em produção

- [ ] chave emitida por tenant, guardada cifrada; nenhuma chave compartilhada
      entre tenants
- [ ] teste automatizado provando que a busca de um cliente **não** devolve o
      outro (RFC §6.1 — sem ele, não vai)
- [ ] tenant ausente no contexto **não** vira chamada: falha fechado
- [ ] toda escrita com `cliente:{ref}`, `sourceReference` e `validTo` por tipo
- [ ] escrita pelo outbox, sobrevivendo ao Brain Sentry fora do ar
- [ ] `obter_cliente_360` responde igual a hoje quando o Brain Sentry cai
- [ ] política de retenção definida em `tenants.settings` (ou decisão
      consciente de não ter)
- [ ] fluxo de remoção do titular testado em dry-run antes do primeiro
      `confirm: true`
