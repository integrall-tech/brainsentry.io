// Business-focused, non-technical help content for every screen.
// Keyed by route path. One entry per screen with pt-BR + en translations.

export interface HelpStep {
  numero: number;
  acao: string;
  esperado?: string;
}

export interface HelpLanguage {
  titulo: string;
  subtitulo: string;
  objetivo: string;
  problema: string;
  comoFunciona: string[];
  fluxoSugerido: HelpStep[];
  regrasChave?: string[];
}

export interface HelpEntry {
  ptBR: HelpLanguage;
  en: HelpLanguage;
}

const pt = (
  titulo: string,
  subtitulo: string,
  objetivo: string,
  problema: string,
  comoFunciona: string[],
  fluxoSugerido: HelpStep[],
  regrasChave?: string[],
): HelpLanguage => ({ titulo, subtitulo, objetivo, problema, comoFunciona, fluxoSugerido, regrasChave });

export const helpContent: Record<string, HelpEntry> = {
  "/app/dashboard": {
    ptBR: pt(
      "Visão geral",
      "O painel da operação",
      "Mostrar num olhar o que a base de conhecimento está produzindo: quantas lembranças existem, quanto está sendo útil e o que precisa de atenção.",
      "Sem esta visão, times acumulam conhecimento sem enxergar o retorno. Daqui você responde rápido: está crescendo? está sendo consultado? está agregando valor?",
      [
        "Cartões com totais de memórias, taxa de uso e taxa de aprovação",
        "Gráficos por categoria e importância",
        "Lista das memórias mais recentes com atalhos para detalhe",
        "Atalhos para as ações mais frequentes do dia a dia",
      ],
      [
        { numero: 1, acao: "Observe os cartões superiores", esperado: "Totais e taxas do momento atual" },
        { numero: 2, acao: "Role até o gráfico por categoria", esperado: "Distribuição do que está sendo capturado" },
        { numero: 3, acao: "Clique em uma memória da lista", esperado: "Abre o detalhe completo" },
      ],
      [
        "A taxa de aprovação mostra se o que foi recuperado realmente ajudou quem perguntou",
        "Memórias 'ativas nas últimas 24h' indicam o conhecimento que está circulando agora",
      ],
    ),
    en: pt(
      "Overview",
      "The operations dashboard",
      "Show at a glance what the knowledge base is producing: how many memories exist, how useful they are, and what needs attention.",
      "Without this view teams hoard knowledge without seeing the payoff. From here you quickly answer: is it growing, is it being consulted, is it adding value?",
      [
        "Cards with memory totals, usage rate and approval rate",
        "Charts by category and importance",
        "Most recent memories list with shortcuts to detail",
        "Shortcuts to the most common daily actions",
      ],
      [
        { numero: 1, acao: "Review the top cards", esperado: "Current totals and rates" },
        { numero: 2, acao: "Scroll to the category chart", esperado: "Distribution of what's being captured" },
        { numero: 3, acao: "Click a memory on the list", esperado: "Opens the full detail" },
      ],
      [
        "The approval rate shows whether retrieved items actually helped the asker",
        "'Active in the last 24h' memories reveal the knowledge that is circulating now",
      ],
    ),
  },

  "/app/memories": {
    ptBR: pt(
      "Memórias",
      "O acervo de lembranças do time",
      "Cadastrar, revisar e organizar o que seus agentes e seu time precisam lembrar. Cada memória é uma ideia, uma decisão, um aprendizado ou um contexto que vale a pena guardar.",
      "Conversas e reuniões produzem conhecimento que some quando muda a rotina, o time ou o canal de comunicação. Aqui esse conhecimento vira acervo consultável.",
      [
        "Crie memórias com título, conteúdo, categoria e importância",
        "Filtre por categoria, tag ou data",
        "Marque como útil ou não útil para ajudar o sistema a aprender",
        "Substitua memórias desatualizadas sem perder o histórico",
      ],
      [
        { numero: 1, acao: "Clique em Nova memória", esperado: "Abre o formulário de cadastro" },
        { numero: 2, acao: "Preencha conteúdo, escolha categoria e importância", esperado: "A memória aparece no topo da lista" },
        { numero: 3, acao: "Abra uma memória e avalie com 👍 ou 👎", esperado: "O peso dela muda em buscas futuras" },
        { numero: 4, acao: "Edite e salve uma memória existente", esperado: "A versão anterior fica registrada no histórico" },
      ],
      [
        "Categoria diz sobre o que é (insight, decisão, alerta, conhecimento, etc.)",
        "Importância (minor / important / critical) guia a prioridade quando o agente vai buscar",
        "Uma memória pode ser substituída por outra: as duas continuam no acervo, mas só a nova está ativa",
      ],
    ),
    en: pt(
      "Memories",
      "Your team's memory vault",
      "Create, review and organise what your agents and team need to remember. Each memory is an idea, a decision, a learning or a context worth keeping.",
      "Conversations and meetings produce knowledge that vanishes when routines, teams or channels change. Here that knowledge becomes a searchable archive.",
      [
        "Create memories with title, content, category and importance",
        "Filter by category, tag or date",
        "Mark as helpful or not helpful to teach the system",
        "Supersede outdated memories without losing history",
      ],
      [
        { numero: 1, acao: "Click New memory", esperado: "The entry form opens" },
        { numero: 2, acao: "Fill in content, pick category and importance", esperado: "The memory appears at the top of the list" },
        { numero: 3, acao: "Open a memory and rate 👍 or 👎", esperado: "Its weight shifts for future retrievals" },
        { numero: 4, acao: "Edit and save an existing memory", esperado: "The previous version stays in history" },
      ],
      [
        "Category tells what it is (insight, decision, warning, knowledge, etc.)",
        "Importance (minor / important / critical) drives priority at retrieval time",
        "A memory can be superseded by another: both stay, only the new one is active",
      ],
    ),
  },

  "/app/search": {
    ptBR: pt(
      "Busca",
      "Encontre o que o time já aprendeu",
      "Buscar memórias por significado, não só por palavra exata. Pergunte em linguagem natural e veja o que a base retorna com a pontuação de relevância.",
      "Uma base grande de conhecimento não ajuda se ninguém acha nada. Aqui qualquer pessoa pergunta como falaria e recebe o que mais se aproxima.",
      [
        "Pergunte em linguagem natural (ex.: como fizemos o rollback no ano passado?)",
        "Filtre por categoria, tag, importância ou intervalo de tempo",
        "Veja a pontuação de relevância de cada resultado",
        "Abra qualquer resultado para ver o conteúdo completo e o contexto",
      ],
      [
        { numero: 1, acao: "Digite uma pergunta no campo de busca", esperado: "Lista de memórias ordenadas por relevância" },
        { numero: 2, acao: "Aplique um filtro de categoria", esperado: "A lista se ajusta mantendo a ordenação" },
        { numero: 3, acao: "Clique no primeiro resultado", esperado: "Abre o detalhe completo" },
      ],
      [
        "A busca combina similaridade semântica e palavra-chave",
        "Resultados antigos pesam menos que recentes quando o tempo é relevante",
        "Respostas marcadas como úteis no passado sobem na próxima consulta",
      ],
    ),
    en: pt(
      "Search",
      "Find what the team already learned",
      "Search memories by meaning, not just exact words. Ask in natural language and see what the base returns with a relevance score.",
      "A big knowledge base is worthless if nobody finds anything. Here anyone asks the way they'd talk and gets the closest matches.",
      [
        "Ask in natural language (e.g. how did we do the rollback last year?)",
        "Filter by category, tag, importance or time range",
        "See a relevance score on every result",
        "Open any result for full content and context",
      ],
      [
        { numero: 1, acao: "Type a question in the search box", esperado: "A list of memories ranked by relevance" },
        { numero: 2, acao: "Apply a category filter", esperado: "The list refines, keeping the ranking" },
        { numero: 3, acao: "Click the top result", esperado: "Opens the full detail" },
      ],
      [
        "Search blends semantic similarity and keyword match",
        "Older items weigh less than recent ones when time matters",
        "Answers marked helpful in the past rise on next query",
      ],
    ),
  },

  "/app/relationships": {
    ptBR: pt(
      "Relacionamentos",
      "Quem se conecta com quem",
      "Declarar e visualizar as ligações entre memórias: quais conversam entre si, quais se complementam, quais se contradizem.",
      "Conhecimento solto não faz rede. Sem conexões explícitas o sistema recupera peças, mas não monta o quebra-cabeça. Aqui você tece a teia.",
      [
        "Liste todas as relações da sua base, filtradas por tipo",
        "Crie uma relação entre duas memórias com um rótulo",
        "Remova relações que não fazem mais sentido",
      ],
      [
        { numero: 1, acao: "Pesquise uma memória no lado esquerdo", esperado: "Encontra a memória de origem" },
        { numero: 2, acao: "Selecione outra memória e declare o tipo de relação", esperado: "A conexão aparece na lista" },
        { numero: 3, acao: "Abra a visão de grafo para ver a teia", esperado: "Você enxerga clusters formando-se" },
      ],
      [
        "Tipos comuns: relacionada_a, depende_de, substitui, contradiz",
        "Uma relação é direcional — importa qual memória é a origem e qual é o destino",
      ],
    ),
    en: pt(
      "Relationships",
      "Who connects to whom",
      "Declare and view the links between memories: which talk to each other, which complement and which contradict.",
      "Loose knowledge isn't a network. Without explicit links the system retrieves pieces but not the puzzle. Here you weave the web.",
      [
        "List every relationship in your base, filtered by type",
        "Create a relation between two memories with a label",
        "Remove relations that no longer hold",
      ],
      [
        { numero: 1, acao: "Search a memory on the left side", esperado: "Finds the source memory" },
        { numero: 2, acao: "Pick another and declare the relation type", esperado: "The connection shows up in the list" },
        { numero: 3, acao: "Open the graph view to see the web", esperado: "Clusters emerge" },
      ],
      [
        "Common types: related_to, depends_on, supersedes, contradicts",
        "A relation is directional — source vs. target matter",
      ],
    ),
  },

  "/app/timeline": {
    ptBR: pt(
      "Linha do tempo",
      "A cronologia do que foi aprendido",
      "Ver o que o time capturou em ordem cronológica, como um diário de aprendizados.",
      "Muitos times perdem a noção de quando aprenderam cada coisa. Esta tela mostra a evolução em formato de linha do tempo, com as mais novas em cima.",
      [
        "Linha do tempo contínua, alternando lados a cada item",
        "Filtros por categoria e nível mínimo de importância",
        "Carregar mais conforme desce para ver o passado",
      ],
      [
        { numero: 1, acao: "Navegue pelos itens do topo", esperado: "As capturas mais recentes aparecem primeiro" },
        { numero: 2, acao: "Escolha um filtro de categoria", esperado: "Só o que interessa continua visível" },
        { numero: 3, acao: "Clique em 'Carregar mais'", esperado: "Mais semanas aparecem abaixo" },
      ],
    ),
    en: pt(
      "Timeline",
      "The chronology of what was learned",
      "See what the team captured in chronological order, like a learning diary.",
      "Most teams lose track of when they learned each thing. This screen shows the evolution as a timeline, newest on top.",
      [
        "Continuous timeline alternating sides per item",
        "Category filter and minimum importance level",
        "Load more as you scroll to uncover the past",
      ],
      [
        { numero: 1, acao: "Browse top items", esperado: "Most recent captures show first" },
        { numero: 2, acao: "Pick a category filter", esperado: "Only the relevant remains visible" },
        { numero: 3, acao: "Click 'Load more'", esperado: "Older weeks appear below" },
      ],
    ),
  },

  "/app/graph/global": {
    ptBR: pt(
      "Grafo Global",
      "O mapa completo da base",
      "Ver todo o acervo como um mapa. Cada ponto é uma memória e cada linha é uma conexão. Cores agrupam temas que o sistema percebeu juntos.",
      "Quando a base tem centenas ou milhares de memórias, listas deixam de ajudar. O mapa mostra bolsões de tema, pontos isolados e pontes entre áreas que precisam conversar.",
      [
        "Cores por comunidade — temas detectados automaticamente",
        "Filtros por categoria e importância aplicam-se ao mapa inteiro",
        "Tamanho do ponto reflete o quanto aquela memória é acessada",
        "Sobreposição opcional mostra onde o feedback do time está baixo",
        "Clique em um ponto para ver detalhes e pular para o ego-grafo",
      ],
      [
        { numero: 1, acao: "Aguarde o mapa carregar", esperado: "Pontos e arestas distribuídos com cores" },
        { numero: 2, acao: "Ative 'Opacidade por feedback'", esperado: "Pontos mais opacos onde a avaliação do time é baixa" },
        { numero: 3, acao: "Clique em um ponto grande", esperado: "Painel à direita mostra o conteúdo e os contadores" },
        { numero: 4, acao: "No painel, toque em 'Abrir Ego-grafo'", esperado: "Você vai para a vizinhança daquela memória" },
      ],
      [
        "Pontos cinza não entraram em nenhuma comunidade — normalmente isolados",
        "Setas vermelhas indicam que uma memória foi substituída por outra",
      ],
    ),
    en: pt(
      "Global Graph",
      "The whole base as a map",
      "See the entire archive as a map. Each dot is a memory and each line is a connection. Colours cluster themes the system picked up.",
      "Once the base has hundreds or thousands of memories, lists stop helping. The map reveals topic pockets, lone islands and bridges between areas that must talk.",
      [
        "Colours by community — themes detected automatically",
        "Category and importance filters apply to the whole map",
        "Dot size reflects how often the memory is accessed",
        "Optional overlay shows where team feedback is weak",
        "Click a dot to see details and jump into its ego graph",
      ],
      [
        { numero: 1, acao: "Wait for the map to load", esperado: "Dots and edges distributed with colours" },
        { numero: 2, acao: "Toggle 'Opacity by feedback'", esperado: "Dimmer dots where team rating is low" },
        { numero: 3, acao: "Click a big dot", esperado: "Right panel shows content and counters" },
        { numero: 4, acao: "From the panel tap 'Open Ego Graph'", esperado: "You jump into that memory's neighbourhood" },
      ],
      [
        "Grey dots never joined a community — usually isolated items",
        "Red arrows mean a memory was replaced by another",
      ],
    ),
  },

  "/app/graph/ego": {
    ptBR: pt(
      "Ego-grafo",
      "A vizinhança de uma lembrança",
      "A partir de uma memória escolhida, enxergar suas vizinhas diretas e indiretas. Útil para investigar o contexto completo de uma decisão ou insight.",
      "Saber que existe uma memória não basta — é preciso ver o que está ao redor dela. Esta visão traz, em um raio de alguns passos, tudo que se liga ao ponto escolhido.",
      [
        "Escolha a memória-semente pelo identificador",
        "Ajuste o alcance (até 4 passos)",
        "Cores indicam a distância do centro — quanto mais perto, mais vivo",
        "Clique em qualquer nó para re-centrar ali",
        "Histórico permite voltar à semente anterior",
      ],
      [
        { numero: 1, acao: "Cole um identificador de memória no campo", esperado: "Botão Explorar fica ativo" },
        { numero: 2, acao: "Clique em Explorar", esperado: "Aparece a semente destacada no centro" },
        { numero: 3, acao: "Aumente para 3 passos", esperado: "A rede cresce e surgem conexões indiretas" },
        { numero: 4, acao: "Clique em um nó distante e confirme o re-centrar", esperado: "A vizinhança é recalculada ao redor dele" },
      ],
      [
        "Passo 0 é a semente — sempre destacada em vermelho",
        "Passo 1 são as vizinhas diretas; passos 2+ são conexões indiretas",
      ],
    ),
    en: pt(
      "Ego Graph",
      "A memory's neighbourhood",
      "From a chosen memory, see its direct and indirect neighbours. Useful to investigate the full context of a decision or insight.",
      "Knowing a memory exists isn't enough — you must see what surrounds it. This view brings, within a few hops, everything tied to the chosen point.",
      [
        "Pick the seed memory by id",
        "Adjust the reach (up to 4 hops)",
        "Colours show distance from the centre — closer is brighter",
        "Click any node to re-centre there",
        "History lets you go back to the previous seed",
      ],
      [
        { numero: 1, acao: "Paste a memory id in the field", esperado: "Explore button becomes active" },
        { numero: 2, acao: "Click Explore", esperado: "Seed appears highlighted at the centre" },
        { numero: 3, acao: "Bump to 3 hops", esperado: "The web grows, indirect links appear" },
        { numero: 4, acao: "Click a far node and re-centre", esperado: "Neighbourhood is recomputed around it" },
      ],
      [
        "Hop 0 is the seed — always highlighted in red",
        "Hop 1 are direct neighbours; 2+ are indirect links",
      ],
    ),
  },

  "/app/graph/timeline": {
    ptBR: pt(
      "Grafo Temporal",
      "O que valia em cada momento",
      "Ver as memórias no tempo e como foram sendo substituídas. Ideal para reconstruir o que o time sabia em determinada data.",
      "Decisões são avaliadas com o conhecimento disponível no momento. Esta visão permite responder: em tal dia, o que estávamos sabendo sobre esse assunto?",
      [
        "Eixo horizontal é o tempo, linhas horizontais separam categorias",
        "Cada ponto é uma memória, posicionada pela data em que foi registrada",
        "Setas vermelhas mostram quando uma memória foi substituída",
        "Janelas pré-definidas: 24 horas, 7 dias, 30 dias ou tudo",
      ],
      [
        { numero: 1, acao: "Escolha a janela 30 dias", esperado: "Os últimos 30 dias aparecem distribuídos" },
        { numero: 2, acao: "Observe as setas vermelhas", esperado: "Indicam memórias obsoletas apontando para suas sucessoras" },
        { numero: 3, acao: "Clique em um ponto desbotado", esperado: "Painel mostra 'Válida até...' com a data de expiração" },
      ],
      [
        "Pontos desbotados foram substituídos — já não valem",
        "A data 'válida a partir de' indica quando a memória passou a valer, não quando foi registrada",
      ],
    ),
    en: pt(
      "Timeline Graph",
      "What was true at each moment",
      "See memories over time and how they were superseded. Great for reconstructing what the team knew on a given date.",
      "Decisions are judged against the knowledge available at the time. This view answers: on that day, what did we know on this topic?",
      [
        "Horizontal axis is time, horizontal lanes separate categories",
        "Each dot is a memory, placed by its recorded date",
        "Red arrows show when a memory was replaced",
        "Preset windows: 24h, 7 days, 30 days or all",
      ],
      [
        { numero: 1, acao: "Pick the 30-day window", esperado: "Last 30 days spread across the axis" },
        { numero: 2, acao: "Watch the red arrows", esperado: "Mark stale memories pointing to their successors" },
        { numero: 3, acao: "Click a faded dot", esperado: "Panel shows 'Valid until' with the expiry date" },
      ],
      [
        "Faded dots were superseded — no longer active",
        "'Valid from' is when it started being true, not when it was recorded",
      ],
    ),
  },

  "/app/decisions": {
    ptBR: pt(
      "Decisões",
      "O registro das escolhas do time",
      "Registrar decisões com cenário, raciocínio, resultado e grau de confiança. Voltar nelas depois e entender por que cada caminho foi escolhido.",
      "A maioria das decisões boas é esquecida em seis meses. Quando surge dúvida parecida, o time repete o debate e às vezes muda de rumo por falta de memória. Aqui isso deixa de acontecer.",
      [
        "Cadastre categoria, cenário, raciocínio e resultado",
        "Defina a confiança no momento (0% a 100%)",
        "Ligue a decisão a uma decisão-pai para formar cadeias causais",
        "Veja precedentes semelhantes por categoria",
      ],
      [
        { numero: 1, acao: "Preencha o formulário de nova decisão", esperado: "A decisão aparece na lista" },
        { numero: 2, acao: "Abra a decisão e clique em 'Ver precedentes'", esperado: "Lista de decisões parecidas no passado" },
        { numero: 3, acao: "Clique em 'Cadeia causal'", esperado: "Árvore de decisões-pai e decisões-filhas" },
      ],
      [
        "Uma decisão pode ser substituída por outra mais tarde — as duas ficam",
        "Decisões com baixa confiança que se confirmam são ótimos aprendizados para revisitar",
      ],
    ),
    en: pt(
      "Decisions",
      "The log of team choices",
      "Record decisions with scenario, reasoning, outcome and confidence. Come back later and understand why each path was picked.",
      "Most good decisions are forgotten in six months. When a similar doubt arises the team re-debates and may change course for lack of memory. Not here.",
      [
        "Capture category, scenario, reasoning and outcome",
        "Set confidence at the moment (0% to 100%)",
        "Link a decision to a parent to form causal chains",
        "See similar precedents by category",
      ],
      [
        { numero: 1, acao: "Fill the new-decision form", esperado: "The decision appears in the list" },
        { numero: 2, acao: "Open it and click 'View precedents'", esperado: "List of similar past decisions" },
        { numero: 3, acao: "Click 'Causal chain'", esperado: "Tree of parent and child decisions" },
      ],
      [
        "A decision can be superseded later — both stay on record",
        "Low-confidence decisions that pan out are great learnings to revisit",
      ],
    ),
  },

  "/app/policies": {
    ptBR: pt(
      "Políticas",
      "Regras que protegem suas decisões",
      "Definir critérios objetivos que toda decisão tem que atender — confiança mínima, campos obrigatórios, categorias permitidas. Assim o time compartilha o mesmo padrão.",
      "Sem política explícita, cada pessoa aprova decisões por instinto e o nível de rigor varia por humor do dia. Aqui os critérios ficam combinados e aplicados automaticamente.",
      [
        "Crie políticas com nome, categoria e nível de severidade",
        "Escolha o tipo de regra (confiança mínima, exige memórias de apoio, etc.)",
        "Liga e desliga políticas sem apagar o histórico",
        "Violações aparecem marcadas na decisão",
      ],
      [
        { numero: 1, acao: "Crie uma política de 'confiança mínima 70%' para categoria deploy", esperado: "Política listada como ativa" },
        { numero: 2, acao: "Registre uma decisão de deploy com confiança 50%", esperado: "Ela aparece com selo de violação" },
        { numero: 3, acao: "Edite a decisão para 80% e salve", esperado: "O selo some" },
      ],
      [
        "Política aplica-se quando a categoria da decisão casa com a categoria da política (ou '*')",
        "Severidade alta pode ser usada como bloqueio; média e baixa como alerta",
      ],
    ),
    en: pt(
      "Policies",
      "Rules that protect your decisions",
      "Set objective criteria every decision must meet — minimum confidence, required fields, allowed categories. That way the team shares one standard.",
      "Without explicit policy, each person approves by gut and rigour varies by mood. Here the criteria are agreed and enforced automatically.",
      [
        "Create policies with name, category and severity",
        "Pick the rule type (minimum confidence, requires supporting memories, etc.)",
        "Enable/disable without deleting history",
        "Violations appear flagged on the decision",
      ],
      [
        { numero: 1, acao: "Create a 'minimum 70% confidence' policy for deploy category", esperado: "Policy listed as active" },
        { numero: 2, acao: "Record a deploy decision at 50% confidence", esperado: "It shows up with a violation badge" },
        { numero: 3, acao: "Edit the decision to 80% and save", esperado: "The badge disappears" },
      ],
      [
        "A policy applies when decision category matches policy category (or '*')",
        "High severity can block; medium and low are warnings",
      ],
    ),
  },

  "/app/events": {
    ptBR: pt(
      "Eventos",
      "Fatos com data e participantes",
      "Registrar acontecimentos concretos — quem, o quê, quando — que se tornam a coluna vertebral da história do sistema.",
      "Decisões explicam o porquê; memórias trazem contexto. Mas os eventos marcam exatamente o que ocorreu. Sem eles é impossível montar uma linha narrativa auditável.",
      [
        "Registre evento com tipo, título, descrição e data",
        "Adicione participantes (pessoas, sistemas, entidades)",
        "Extraia eventos de um texto livre usando inteligência",
        "Filtre por tipo, data ou participante",
      ],
      [
        { numero: 1, acao: "Crie um evento do tipo DEPLOY com a data de hoje", esperado: "Evento aparece na listagem" },
        { numero: 2, acao: "Abra 'Extrair eventos de texto' e cole uma ata", esperado: "O sistema sugere eventos para você aprovar" },
        { numero: 3, acao: "Clique em um evento para ver seus participantes", esperado: "Detalhe mostra quem esteve envolvido" },
      ],
      [
        "Participantes podem ser pessoas, equipes ou sistemas — todos com papel declarado",
        "Um evento pode estar ligado à memória de origem (ex.: a ata que o gerou)",
      ],
    ),
    en: pt(
      "Events",
      "Facts with date and participants",
      "Record concrete happenings — who, what, when — that become the backbone of the system's story.",
      "Decisions explain why; memories bring context. Events mark what actually happened. Without them there's no auditable narrative.",
      [
        "Record event with type, title, description and date",
        "Add participants (people, systems, entities)",
        "Extract events from free text with AI",
        "Filter by type, date or participant",
      ],
      [
        { numero: 1, acao: "Create a DEPLOY event with today's date", esperado: "Event shows up in the list" },
        { numero: 2, acao: "Open 'Extract events from text' and paste minutes", esperado: "The system suggests events to approve" },
        { numero: 3, acao: "Click an event to see its participants", esperado: "Detail shows who was involved" },
      ],
      [
        "Participants can be people, teams or systems — each with a declared role",
        "An event can link to the source memory (e.g. the minutes that generated it)",
      ],
    ),
  },

  "/app/reasoning": {
    ptBR: pt(
      "Raciocínio",
      "Hipóteses para explicar o que observamos",
      "A partir de uma decisão, pedir ao sistema que levante hipóteses plausíveis sobre as causas, com base nas memórias e eventos que ele conhece.",
      "Nem sempre dá para saber por que algo aconteceu. A função é gerar explicações candidatas, ranqueadas por confiança, para acelerar a investigação.",
      [
        "Selecione a decisão a investigar",
        "Opcionalmente escreva a pergunta que você quer responder",
        "Receba uma lista de hipóteses com grau de confiança",
        "Cada hipótese traz as memórias e entidades que a sustentam",
      ],
      [
        { numero: 1, acao: "Cole o identificador de uma decisão", esperado: "A decisão é carregada" },
        { numero: 2, acao: "Digite 'por que tivemos atraso?' e envie", esperado: "Aparece lista de hipóteses" },
        { numero: 3, acao: "Expanda a hipótese mais provável", esperado: "Vê as memórias que a sustentam" },
      ],
      [
        "Hipóteses são candidatos, não verdades — use como ponto de partida",
        "Mais evidências por trás = mais confiança",
      ],
    ),
    en: pt(
      "Reasoning",
      "Hypotheses to explain what we see",
      "From a decision, ask the system to surface plausible causes based on memories and events it already knows.",
      "You can't always tell why something happened. This generates candidate explanations, ranked by confidence, to speed up investigation.",
      [
        "Pick the decision to investigate",
        "Optionally write the question you want answered",
        "Get a list of hypotheses with confidence scores",
        "Each one cites the memories and entities backing it",
      ],
      [
        { numero: 1, acao: "Paste a decision id", esperado: "The decision loads" },
        { numero: 2, acao: "Type 'why did we delay?' and submit", esperado: "Ranked list of hypotheses" },
        { numero: 3, acao: "Expand the top one", esperado: "You see the memories behind it" },
      ],
      [
        "Hypotheses are candidates, not truths — start there, don't end there",
        "More evidence behind = more confidence",
      ],
    ),
  },

  "/app/provenance": {
    ptBR: pt(
      "Proveniência",
      "Quem fez o quê, quando e porquê",
      "Exportar o registro completo de origens — toda memória, decisão e evento com o responsável, a data e a cadeia que o produziu. Pronto para auditoria.",
      "Em contextos regulados (financeiro, saúde, jurídico) a rastreabilidade deixa de ser luxo. Aqui você gera o pacote de proveniência em um formato aberto e aceito.",
      [
        "Exportar em formato Turtle (texto) ou JSON-LD",
        "Filtrar por intervalo de datas ou categoria",
        "Ver cada ação com ator, tipo, data e o item afetado",
        "Baixar o pacote para enviar ao auditor",
      ],
      [
        { numero: 1, acao: "Escolha o período (últimos 30 dias)", esperado: "Contagem de atividades aparece no topo" },
        { numero: 2, acao: "Clique em Exportar em Turtle", esperado: "Baixa um arquivo .ttl pronto" },
        { numero: 3, acao: "Abra o arquivo no seu editor", esperado: "Cada linha descreve um ato com seu responsável" },
      ],
      [
        "O formato segue um padrão aberto usado por auditoria e arquivística",
        "Nada é apagado — mesmo itens removidos pelo usuário ficam no histórico",
      ],
    ),
    en: pt(
      "Provenance",
      "Who did what, when and why",
      "Export the full origin log — every memory, decision and event with actor, date and causal chain. Audit-ready.",
      "In regulated spaces (finance, health, legal) traceability stops being nice-to-have. Here you generate a provenance package in an open, accepted format.",
      [
        "Export as Turtle (text) or JSON-LD",
        "Filter by date range or category",
        "See each action with actor, type, date and affected item",
        "Download the package and hand it to the auditor",
      ],
      [
        { numero: 1, acao: "Pick the period (last 30 days)", esperado: "Activity count appears on top" },
        { numero: 2, acao: "Click Export as Turtle", esperado: "Downloads a ready .ttl file" },
        { numero: 3, acao: "Open it in your editor", esperado: "Each line describes an act with its actor" },
      ],
      [
        "The format follows an open standard used in audit and archiving",
        "Nothing is erased — even user-removed items stay on record",
      ],
    ),
  },

  "/app/console": {
    ptBR: pt(
      "Console",
      "A conversa direta com a memória",
      "Operar a base num modo conversacional: lembrar algo novo, recuperar o que existe, melhorar o acervo e esquecer o que não serve mais.",
      "Às vezes você precisa de uma interface simples, quase um chat, para testar ideias ou capturar rapidamente. Este é o atalho mais curto entre a sua cabeça e o acervo.",
      [
        "Lembrar: cole o texto, opcionalmente associado a uma sessão",
        "Recuperar: faça uma pergunta e veja o que o sistema traz",
        "Melhorar: deixa o sistema limpar contradições e duplicidades",
        "Esquecer: descarte o que não faz mais sentido",
      ],
      [
        { numero: 1, acao: "No campo lembrar, escreva uma anotação", esperado: "Confirma que guardou com identificador" },
        { numero: 2, acao: "Faça uma pergunta no campo recuperar", esperado: "Lista de memórias mais próximas, com a roteadora escolhida" },
        { numero: 3, acao: "Clique em Melhorar (simulação)", esperado: "Mostra quantos itens seriam afetados" },
      ],
      [
        "Uma sessão separa memórias do mesmo contexto (ex.: uma conversa específica)",
        "A 'roteadora' escolhe a melhor estratégia de busca pra pergunta que você fez",
      ],
    ),
    en: pt(
      "Console",
      "A direct chat with the memory",
      "Run the base conversationally: remember something new, recall what's there, improve the archive and forget what no longer fits.",
      "Sometimes you need a plain interface, almost a chat, to test ideas or capture quickly. This is the shortest path from your head to the archive.",
      [
        "Remember: paste text, optionally tied to a session",
        "Recall: ask a question and see what comes back",
        "Improve: let the system clean contradictions and duplicates",
        "Forget: discard what's no longer relevant",
      ],
      [
        { numero: 1, acao: "In the remember box, type a note", esperado: "Confirms saved with an id" },
        { numero: 2, acao: "Ask a question in the recall box", esperado: "List of closest memories with the router's pick" },
        { numero: 3, acao: "Click Improve (dry run)", esperado: "Shows how many items would be affected" },
      ],
      [
        "A session isolates memories from the same context (e.g. a specific chat)",
        "The router picks the best search strategy for the question asked",
      ],
    ),
  },

  "/app/traces": {
    ptBR: pt(
      "Traços de Agente",
      "O que o agente fez, por quê e com qual resultado",
      "Acompanhar cada chamada feita por um agente: o que ele perguntou, o que recebeu de contexto e o que entregou no final.",
      "Quando um agente age mal, o primeiro instinto é culpar o modelo. Mas quase sempre o problema está no que ele pediu, no que recebeu ou em como formou a resposta. Esta visão deixa o trabalho do agente transparente.",
      [
        "Lista cronológica das chamadas",
        "Filtros por função de origem, status e palavra-chave",
        "Expandir para ver entrada, contexto injetado e saída",
        "Estatísticas de sucesso e falha por período",
      ],
      [
        { numero: 1, acao: "Veja os traços mais recentes", esperado: "Lista com hora e função de origem" },
        { numero: 2, acao: "Aplique filtro 'só erros'", esperado: "Só chamadas que falharam ficam visíveis" },
        { numero: 3, acao: "Expanda um traço com erro", esperado: "Mostra a mensagem, o contexto recebido e a saída" },
      ],
      [
        "Traços ficam anonimizados por padrão — dados sensíveis são mascarados",
        "Um traço com retorno vazio costuma indicar pergunta mal formulada ao agente",
      ],
    ),
    en: pt(
      "Agent Traces",
      "What the agent did, why and how",
      "Follow every agent call: what it asked, what context it received and what it delivered.",
      "When an agent misbehaves the first instinct is to blame the model. Usually the fault is in the ask, the context it got or how it phrased the answer. This view makes the agent's work transparent.",
      [
        "Chronological call list",
        "Filters by origin function, status and keyword",
        "Expand to see input, injected context and output",
        "Success/failure stats per period",
      ],
      [
        { numero: 1, acao: "Check the most recent traces", esperado: "List with time and origin function" },
        { numero: 2, acao: "Apply 'errors only' filter", esperado: "Only failed calls remain visible" },
        { numero: 3, acao: "Expand a failed trace", esperado: "Shows message, context received and output" },
      ],
      [
        "Traces are anonymised by default — sensitive data is masked",
        "An empty return often means a poorly framed question to the agent",
      ],
    ),
  },

  "/app/extraction": {
    ptBR: pt(
      "Lab de Extração",
      "Teste como o sistema entende textos",
      "Um laboratório para colar um texto e ver o que o sistema extrai em termos de entidades, relações e tripletos. Ajuda a calibrar antes de rodar em produção.",
      "Qualidade da extração é crítica para a qualidade do acervo. Testar em um pedaço de texto antes de ingerir milhares economiza esforço e evita ruído.",
      [
        "Aba Tripletos: vê sujeito → predicado → objeto",
        "Aba Cascata: mostra entidades e relações passo a passo",
        "Compare resultados entre estratégias",
        "Copie o JSON resultante para revisar com o time",
      ],
      [
        { numero: 1, acao: "Cole um trecho de relatório de reunião", esperado: "O editor aceita o texto" },
        { numero: 2, acao: "Clique em Extrair tripletos", esperado: "Tabela com sujeito/predicado/objeto" },
        { numero: 3, acao: "Alterne para Cascata e extraia de novo", esperado: "Vê entidades e depois relações separadamente" },
      ],
      [
        "A estratégia em cascata costuma errar menos em textos longos",
        "Relações que não aparecem no texto original não devem ser inventadas — revise antes de salvar",
      ],
    ),
    en: pt(
      "Extraction Lab",
      "Test how the system parses text",
      "A lab to paste text and see what the system extracts as entities, relations and triples. Helps you calibrate before production runs.",
      "Extraction quality is critical to archive quality. Testing on a sample before ingesting thousands saves effort and avoids noise.",
      [
        "Triples tab: subject → predicate → object",
        "Cascade tab: entities and relations step by step",
        "Compare results across strategies",
        "Copy the resulting JSON to review with the team",
      ],
      [
        { numero: 1, acao: "Paste a meeting-notes snippet", esperado: "The editor accepts the text" },
        { numero: 2, acao: "Click Extract triples", esperado: "Table with subject/predicate/object" },
        { numero: 3, acao: "Switch to Cascade and re-run", esperado: "Entities first, then relations" },
      ],
      [
        "Cascade usually errs less on long texts",
        "Relations not present in the original shouldn't be invented — review before saving",
      ],
    ),
  },

  "/app/ontology": {
    ptBR: pt(
      "Ontologia",
      "O vocabulário compartilhado do domínio",
      "Definir os tipos de entidade e os nomes de relação que fazem sentido no seu mundo, para que a base toda use o mesmo vocabulário.",
      "Sem ontologia, cada extração inventa nomes parecidos para a mesma coisa — 'Postgres', 'PostgreSQL', 'postgres-db'. Com vocabulário controlado, a base vira um grafo coerente.",
      [
        "Carregue uma ontologia existente ou cole em formato JSON",
        "Resolva um texto livre contra o vocabulário — o sistema sugere o termo canônico",
        "Veja os tipos de entidade e relações permitidos",
      ],
      [
        { numero: 1, acao: "Abra o editor JSON e cole um exemplo pequeno", esperado: "A ontologia é carregada" },
        { numero: 2, acao: "No campo de teste, escreva 'postgres' e resolva", esperado: "Mostra o termo canônico escolhido" },
      ],
      [
        "O sistema prefere termos da ontologia quando o nome do texto é ambíguo",
        "Uma ontologia vazia = 'extração livre'. Uma ontologia rica = consistência maior",
      ],
    ),
    en: pt(
      "Ontology",
      "The domain's shared vocabulary",
      "Define the entity types and relation names that make sense in your world, so the whole base shares one vocabulary.",
      "Without ontology each extraction invents near-names for the same thing — 'Postgres', 'PostgreSQL', 'postgres-db'. Controlled vocabulary turns the base into a coherent graph.",
      [
        "Load an existing ontology or paste JSON",
        "Resolve free text against the vocabulary — the system proposes the canonical term",
        "View the allowed entity types and relations",
      ],
      [
        { numero: 1, acao: "Open the JSON editor and paste a small example", esperado: "Ontology loads" },
        { numero: 2, acao: "Type 'postgres' in the test field and resolve", esperado: "Canonical term is returned" },
      ],
      [
        "The system prefers ontology terms when free text is ambiguous",
        "Empty ontology = free extraction. Rich ontology = more consistency",
      ],
    ),
  },

  "/app/session-cache": {
    ptBR: pt(
      "Cache de Sessão",
      "Conversas ativas, em alta velocidade",
      "Manter o histórico de conversas em cache rápido, sem gravar cada mensagem no acervo permanente. Depois, o que for relevante vira memória de verdade.",
      "Nem toda troca merece virar memória. Uma conversa de 40 mensagens normalmente contém 3 ou 4 aprendizados que merecem ficar. O cache dá agilidade para a conversa e o 'cognificar' cristaliza só o essencial.",
      [
        "Lista as sessões ativas e recentes",
        "Abra uma sessão e veja as interações trocadas",
        "Dispare a cognificação: o sistema escolhe o que vai para o acervo permanente",
        "Sessões encerradas expiram do cache automaticamente",
      ],
      [
        { numero: 1, acao: "Abra uma sessão da lista", esperado: "Mostra as mensagens na ordem trocada" },
        { numero: 2, acao: "Clique em Cognificar", esperado: "O sistema gera um resumo e memórias derivadas" },
        { numero: 3, acao: "Volte ao acervo e encontre as novas memórias", esperado: "Elas aparecem com origem ligada à sessão" },
      ],
      [
        "Conversas sem valor não precisam virar memória — basta deixar expirar",
        "A cognificação pode ser revisada antes de ser aprovada",
      ],
    ),
    en: pt(
      "Session Cache",
      "Active chats at high speed",
      "Keep conversation history in a fast cache without writing every message into the permanent archive. Later, what matters becomes a real memory.",
      "Not every exchange deserves to be a memory. A 40-message chat usually contains 3 or 4 learnings worth keeping. The cache gives speed; 'cognify' crystallises only the essential.",
      [
        "Lists active and recent sessions",
        "Open a session to see the exchanges",
        "Trigger cognify: the system picks what goes permanent",
        "Closed sessions expire automatically",
      ],
      [
        { numero: 1, acao: "Open a session from the list", esperado: "Messages appear in order" },
        { numero: 2, acao: "Click Cognify", esperado: "The system produces a summary and derived memories" },
        { numero: 3, acao: "Return to the archive and find the new memories", esperado: "They show up linked to the session as origin" },
      ],
      [
        "Low-value chats don't need to become memories — let them expire",
        "Cognify output can be reviewed before being approved",
      ],
    ),
  },

  "/app/actions": {
    ptBR: pt(
      "Ações & Leases",
      "Coordenação entre agentes",
      "Fila onde vários agentes podem pegar tarefas, executar e devolver resultado, sem pisarem no pé uns dos outros.",
      "Quando dois ou mais agentes tentam resolver a mesma tarefa ao mesmo tempo, sobram retrabalho e contradição. A lease (posse temporária) garante que um agente por vez cuida de cada ação.",
      [
        "Crie uma ação com tipo e payload",
        "Um agente reivindica a lease — os outros veem a ação bloqueada",
        "Agente devolve o resultado e libera a lease",
        "Lease expira se o agente sumir — outro assume",
      ],
      [
        { numero: 1, acao: "Crie uma nova ação de 'revisar PR'", esperado: "Ação aparece como pendente" },
        { numero: 2, acao: "Clique em Reivindicar lease", esperado: "Status muda para 'em execução' com o agente responsável" },
        { numero: 3, acao: "Conclua a ação com resultado", esperado: "Ela fica registrada como concluída" },
      ],
      [
        "Tempo de lease é limitado — se estourar sem conclusão, outro agente pode assumir",
        "Ações concluídas ficam no histórico para auditoria",
      ],
    ),
    en: pt(
      "Actions & Leases",
      "Agent coordination",
      "A queue where multiple agents pick tasks, run and return results, without stepping on each other.",
      "When two agents try the same task at once you get rework and contradictions. The lease (temporary ownership) makes sure one agent at a time handles each action.",
      [
        "Create an action with type and payload",
        "An agent claims the lease — others see it blocked",
        "The agent returns the result and releases the lease",
        "Lease expires if the agent vanishes — another takes over",
      ],
      [
        { numero: 1, acao: "Create a 'review PR' action", esperado: "Action appears pending" },
        { numero: 2, acao: "Click Claim lease", esperado: "Status becomes 'running' with the owning agent" },
        { numero: 3, acao: "Complete the action with a result", esperado: "It stays on record as completed" },
      ],
      [
        "Lease time is bounded — if it blows without completion another agent takes it",
        "Completed actions stay on record for audit",
      ],
    ),
  },

  "/app/mesh": {
    ptBR: pt(
      "Sincronização Mesh",
      "Acervos conversando entre si",
      "Conectar duas ou mais instâncias do sistema para trocar memórias escolhidas entre times, filiais ou ambientes.",
      "Conhecimento trancado numa instância só não serve o grupo. A sincronização em malha permite que times independentes compartilhem partes do acervo sem fundir tudo num lugar só.",
      [
        "Registre um peer (outra instância)",
        "Escolha o escopo: memórias, decisões ou eventos",
        "Dispare sincronização manual ou agendada",
        "Veja quantos itens foram enviados e recebidos",
      ],
      [
        { numero: 1, acao: "Cadastre um peer novo com URL e credencial", esperado: "Peer aparece na lista como conectado" },
        { numero: 2, acao: "Dispare sincronização de memórias", esperado: "Relatório mostra enviadas, recebidas e fundidas" },
        { numero: 3, acao: "Veja o histórico de sincronizações", esperado: "Cada rodada fica registrada" },
      ],
      [
        "A sincronização não sobrescreve — conflitos ficam visíveis para resolução manual",
        "Cada peer pode ter escopo diferente (ex.: só memórias, sem decisões)",
      ],
    ),
    en: pt(
      "Mesh Sync",
      "Archives talking to each other",
      "Connect two or more instances to exchange selected memories between teams, branches or environments.",
      "Knowledge locked in a single instance doesn't serve the group. Mesh sync lets independent teams share slices without merging everything.",
      [
        "Register a peer (another instance)",
        "Pick scope: memories, decisions or events",
        "Trigger manual or scheduled sync",
        "See how many items were sent and received",
      ],
      [
        { numero: 1, acao: "Add a peer with URL and credential", esperado: "Peer shows as connected" },
        { numero: 2, acao: "Trigger a memory sync", esperado: "Report shows sent, received and merged" },
        { numero: 3, acao: "Check sync history", esperado: "Each round is recorded" },
      ],
      [
        "Sync doesn't overwrite — conflicts surface for manual resolution",
        "Each peer can have a different scope (e.g. memories only, no decisions)",
      ],
    ),
  },

  "/app/batch-search": {
    ptBR: pt(
      "Busca em Lote",
      "Várias perguntas de uma vez",
      "Fazer um bloco de perguntas ao acervo de uma vez e comparar os melhores resultados em uma matriz.",
      "Ao elaborar um relatório ou briefing, você tem muitas dúvidas parecidas. Em vez de pesquisar uma a uma, roda o bloco e enxerga o panorama. Economiza horas.",
      [
        "Cole ou digite várias perguntas, uma por linha",
        "Receba uma matriz: cada memória x cada pergunta, com pontuação",
        "Identifique rapidamente as memórias que resolvem várias perguntas",
      ],
      [
        { numero: 1, acao: "Escreva três perguntas diferentes", esperado: "Botão Buscar fica ativo" },
        { numero: 2, acao: "Clique em Buscar", esperado: "Matriz aparece com pontuações" },
        { numero: 3, acao: "Ordene por 'máximo' ou 'média'", esperado: "As memórias mais úteis sobem no ranking" },
      ],
    ),
    en: pt(
      "Batch Search",
      "Many questions at once",
      "Run a block of questions against the archive at once and compare best hits in a score matrix.",
      "Writing a report or brief, you have many similar doubts. Instead of searching one by one, run the block and see the whole picture. Saves hours.",
      [
        "Paste or type multiple questions, one per line",
        "Get a matrix: memory × question, scored",
        "Quickly spot memories that answer several questions",
      ],
      [
        { numero: 1, acao: "Write three different questions", esperado: "Search button becomes active" },
        { numero: 2, acao: "Click Search", esperado: "Matrix with scores appears" },
        { numero: 3, acao: "Sort by 'max' or 'mean'", esperado: "Most useful memories rise to the top" },
      ],
    ),
  },

  "/app/audit": {
    ptBR: pt(
      "Auditoria",
      "O diário do que aconteceu no sistema",
      "Conferir toda operação relevante: quem criou, editou, apagou, acessou e quando.",
      "Confiança no sistema depende de poder responder 'como chegamos neste estado?'. O diário aqui é a resposta.",
      [
        "Lista cronológica de operações",
        "Filtros por intervalo de data, tipo de operação e usuário",
        "Exportação para envio a auditor externo",
      ],
      [
        { numero: 1, acao: "Abra a lista e filtre pelos últimos 7 dias", esperado: "Só operações recentes aparecem" },
        { numero: 2, acao: "Filtre por tipo 'memória.substituída'", esperado: "Vê só esse tipo" },
      ],
      [
        "O diário não pode ser apagado — é a garantia da auditoria",
        "Eventos sensíveis podem ser marcados para revisão semanal",
      ],
    ),
    en: pt(
      "Audit",
      "The system's activity log",
      "Review every relevant operation: who created, edited, deleted, accessed and when.",
      "Trust in the system depends on answering 'how did we get to this state?'. The log here is that answer.",
      [
        "Chronological operation list",
        "Filters by date range, operation type and user",
        "Export for an external auditor",
      ],
      [
        { numero: 1, acao: "Open the list, filter last 7 days", esperado: "Only recent operations show" },
        { numero: 2, acao: "Filter by type 'memory.superseded'", esperado: "Only that type remains" },
      ],
      [
        "The log cannot be deleted — that's the audit guarantee",
        "Sensitive events can be flagged for weekly review",
      ],
    ),
  },

  "/app/users": {
    ptBR: pt(
      "Usuários",
      "Quem pode operar a base",
      "Gerenciar pessoas com acesso: convidar, remover, alterar papel e desativar temporariamente.",
      "Um acervo de conhecimento só é útil com acesso controlado. Aqui você decide quem pode ler, quem pode criar e quem pode administrar.",
      [
        "Convide novos usuários por e-mail",
        "Atribua papel: administrador, editor ou leitor",
        "Desative acesso sem perder o histórico",
      ],
      [
        { numero: 1, acao: "Clique em Novo usuário e informe e-mail", esperado: "Convite é enviado" },
        { numero: 2, acao: "Mude o papel de um usuário", esperado: "Alteração aparece na lista" },
      ],
      [
        "Administradores podem alterar papéis; editores não",
        "Desativar preserva o histórico; excluir apaga o vínculo, mas registros ficam",
      ],
    ),
    en: pt(
      "Users",
      "Who can operate the base",
      "Manage people with access: invite, remove, change role and temporarily deactivate.",
      "A knowledge base is only useful with controlled access. Here you decide who can read, who can create and who can administer.",
      [
        "Invite new users by email",
        "Assign role: admin, editor or reader",
        "Deactivate access without losing history",
      ],
      [
        { numero: 1, acao: "Click New user and enter email", esperado: "Invite is sent" },
        { numero: 2, acao: "Change a user's role", esperado: "Change appears in the list" },
      ],
      [
        "Admins can change roles; editors cannot",
        "Deactivating preserves history; removing breaks the link but records stay",
      ],
    ),
  },

  "/app/tenants": {
    ptBR: pt(
      "Tenants",
      "Ambientes separados por cliente ou projeto",
      "Operar múltiplos espaços independentes dentro da mesma plataforma — um acervo não vê o outro.",
      "Quando um serviço atende vários clientes ou projetos, misturar dados é risco jurídico e ruído operacional. Cada tenant é um mundo à parte.",
      [
        "Liste tenants existentes",
        "Crie novo tenant com nome, slug e limites",
        "Desative um tenant sem apagar o acervo",
      ],
      [
        { numero: 1, acao: "Clique em Novo tenant", esperado: "Formulário abre" },
        { numero: 2, acao: "Preencha e salve", esperado: "Tenant aparece ativo na lista" },
      ],
      [
        "Usuário pode pertencer a um ou mais tenants — mas só opera um por vez",
        "Limites (memórias, usuários) ajudam a controlar custos quando há muitos clientes",
      ],
    ),
    en: pt(
      "Tenants",
      "Separate spaces per client or project",
      "Run multiple independent spaces on the same platform — one archive does not see the other.",
      "When a service covers many clients or projects, mixing data is a legal risk and operational noise. Each tenant is its own world.",
      [
        "List existing tenants",
        "Create a new tenant with name, slug and limits",
        "Deactivate a tenant without deleting its archive",
      ],
      [
        { numero: 1, acao: "Click New tenant", esperado: "Form opens" },
        { numero: 2, acao: "Fill in and save", esperado: "Tenant appears active in the list" },
      ],
      [
        "A user may belong to multiple tenants — but works in one at a time",
        "Limits (memories, users) control costs when there are many clients",
      ],
    ),
  },

  "/app/configuration": {
    ptBR: pt(
      "Configurações",
      "Ajustes gerais do sistema",
      "Definir parâmetros que afetam o comportamento do acervo: retenção, notificações, integrações, identidade visual.",
      "Cada time tem preferências diferentes — cadência de limpeza automática, política de arquivamento, destino das notificações. Aqui essas escolhas ficam em um lugar só.",
      [
        "Agrupamento por área (geral, retenção, notificações, integrações)",
        "Mudanças entram em vigor imediatamente",
        "Campos obrigatórios são destacados em vermelho",
      ],
      [
        { numero: 1, acao: "Abra a área de Retenção", esperado: "Mostra opções de TTL e limpeza automática" },
        { numero: 2, acao: "Altere um parâmetro e salve", esperado: "Toast de confirmação aparece" },
      ],
      [
        "Alterações em retenção podem remover memórias — revise antes de salvar",
        "Notificações usam o e-mail cadastrado no perfil do operador",
      ],
    ),
    en: pt(
      "Configuration",
      "General system settings",
      "Set parameters that shape the archive: retention, notifications, integrations, visual identity.",
      "Each team has different preferences — cleanup cadence, archive policy, notification targets. Here those choices live in one place.",
      [
        "Grouped by area (general, retention, notifications, integrations)",
        "Changes take effect immediately",
        "Required fields are highlighted in red",
      ],
      [
        { numero: 1, acao: "Open the Retention area", esperado: "Shows TTL and auto-cleanup options" },
        { numero: 2, acao: "Change a parameter and save", esperado: "Confirmation toast appears" },
      ],
      [
        "Retention changes may remove memories — review before saving",
        "Notifications use the email in the operator's profile",
      ],
    ),
  },

  "/app/diagnostics": {
    ptBR: pt(
      "Diagnóstico do Sistema",
      "Tudo está funcionando?",
      "Saber, em segundos, se algum componente do sistema está degradado ou fora do ar — antes que o usuário final note.",
      "Quando uma busca demora muito, um login falha ou um agente não responde, você precisa de uma resposta rápida: é a base de dados, o cache, o provedor de IA, a rede? Esta tela checa todos os componentes e aponta exatamente o que está estranho.",
      [
        "Cada componente externo (banco, cache, grafo, IA) é testado em paralelo",
        "Cada check leva até alguns segundos e mostra latência, mensagem e dica",
        "O resultado agregado vira uma cor: verde, amarelo (atenção) ou vermelho (falha)",
        "É um espelho do comando 'brainsentry doctor' do terminal — mesmo motor, mesmo critério",
      ],
      [
        { numero: 1, acao: "Abra esta tela quando algo parecer fora do ar", esperado: "Carrega o relatório atual em poucos segundos" },
        { numero: 2, acao: "Procure linhas em vermelho ou amarelo", esperado: "Cada uma traz um detalhe e uma dica de ação" },
        { numero: 3, acao: "Clique em 'Rodar novamente' depois de aplicar correções", esperado: "Status muda para verde se o problema foi resolvido" },
      ],
      [
        "Vermelho não é alarme falso — costuma indicar perda real de funcionalidade",
        "Amarelo costuma ser tolerável (cache ausente, por exemplo) mas vale investigar",
        "Use junto com seus dashboards de monitoramento, não no lugar deles",
      ],
    ),
    en: pt(
      "System Diagnostics",
      "Is everything actually working?",
      "Know in seconds whether any system component is degraded or down — before the end user notices.",
      "When a search lags, a login fails or an agent stops responding, you need a quick answer: is it the database, the cache, the AI provider, the network? This screen probes all components and points to exactly what is off.",
      [
        "Each external dependency (DB, cache, graph, AI) is tested in parallel",
        "Each check shows latency, message and a hint",
        "Aggregate result is a color: green, amber (warning), red (failure)",
        "Mirror of the 'brainsentry doctor' CLI — same engine, same criteria",
      ],
      [
        { numero: 1, acao: "Open this screen when something feels off", esperado: "Loads the current report in seconds" },
        { numero: 2, acao: "Look for red or amber rows", esperado: "Each carries a detail and an action hint" },
        { numero: 3, acao: "Click 'Run again' after applying fixes", esperado: "Turns green when resolved" },
      ],
      [
        "Red is rarely a false alarm — usually a real loss of functionality",
        "Amber is often tolerable (missing cache, for example) but worth checking",
        "Pair with your monitoring dashboards, not as a replacement",
      ],
    ),
  },

  "/app/analytics": {
    ptBR: pt(
      "Analytics",
      "Quão bem a busca está servindo o time",
      "Medir a qualidade do sistema: acertos, relevância, tempo gasto e comparar versões entre si.",
      "Falar em qualidade sem números é opinião. Esta tela traz métricas padronizadas para saber se a busca está boa, está piorando ou pode ser otimizada.",
      [
        "Métricas de recuperação (recall@K, NDCG, MRR)",
        "Comparação entre rodadas de benchmark",
        "Gráficos de tendência ao longo do tempo",
      ],
      [
        { numero: 1, acao: "Abra o painel de benchmark", esperado: "Métricas atuais aparecem em destaque" },
        { numero: 2, acao: "Compare com a rodada anterior", esperado: "Sinal (+/-) mostra se melhorou ou piorou" },
      ],
      [
        "Piora em métricas pode indicar acúmulo de memórias obsoletas — verifique o auto-esquecer",
        "Ganhos vêm de mais feedback do time, não só de mudanças de sistema",
      ],
    ),
    en: pt(
      "Analytics",
      "How well search serves the team",
      "Measure system quality: hits, relevance, time spent, and compare versions.",
      "Talking about quality without numbers is opinion. This screen brings standard metrics to know if search is good, declining or can be tuned.",
      [
        "Retrieval metrics (recall@K, NDCG, MRR)",
        "Benchmark run comparison",
        "Trend charts over time",
      ],
      [
        { numero: 1, acao: "Open the benchmark panel", esperado: "Current metrics stand out" },
        { numero: 2, acao: "Compare with the previous run", esperado: "Arrows show better/worse" },
      ],
      [
        "Declining metrics may signal stale memory buildup — check auto-forget",
        "Gains come from more team feedback, not only system changes",
      ],
    ),
  },

  "/app/profile": {
    ptBR: pt(
      "Perfil",
      "Seus dados e preferências",
      "Ajustar nome, e-mail, idioma e notificações do seu próprio usuário.",
      "Preferências pessoais não devem afetar os colegas. Aqui ficam isoladas por usuário.",
      [
        "Editar nome e e-mail",
        "Escolher idioma da interface (pt-BR / en)",
        "Configurar frequência de notificações",
      ],
      [
        { numero: 1, acao: "Altere o idioma para inglês", esperado: "Interface muda ao vivo" },
        { numero: 2, acao: "Salve e recarregue", esperado: "Preferência se mantém" },
      ],
    ),
    en: pt(
      "Profile",
      "Your data and preferences",
      "Adjust name, email, language and notifications for your own user.",
      "Personal preferences shouldn't affect colleagues. Here they're isolated per user.",
      [
        "Edit name and email",
        "Pick interface language (pt-BR / en)",
        "Set notification frequency",
      ],
      [
        { numero: 1, acao: "Switch language to Portuguese", esperado: "Interface changes live" },
        { numero: 2, acao: "Save and reload", esperado: "Preference persists" },
      ],
    ),
  },

  "/app/playground": {
    ptBR: pt(
      "Playground",
      "Experimente sem risco",
      "Ambiente de testes para ver como o sistema responde a perguntas, prompts ou payloads sem afetar o acervo real.",
      "Ideias novas precisam ser testadas antes de virar processo. Aqui você experimenta com tranquilidade — nada do que roda aqui é salvo.",
      [
        "Campo de entrada livre para prompts e payloads",
        "Resposta aparece lado a lado com a entrada",
        "Histórico de testes da sessão atual",
      ],
      [
        { numero: 1, acao: "Cole um prompt de teste", esperado: "Botão Rodar fica ativo" },
        { numero: 2, acao: "Execute", esperado: "Resposta aparece ao lado" },
      ],
    ),
    en: pt(
      "Playground",
      "Experiment safely",
      "A sandbox to see how the system answers questions, prompts or payloads without touching the real archive.",
      "New ideas must be tested before becoming process. Here you try things freely — nothing ran here is saved.",
      [
        "Free input field for prompts and payloads",
        "Response shown side by side with input",
        "Session history of attempts",
      ],
      [
        { numero: 1, acao: "Paste a test prompt", esperado: "Run button becomes active" },
        { numero: 2, acao: "Run", esperado: "Response appears beside it" },
      ],
    ),
  },

  "/app/connectors": {
    ptBR: pt(
      "Conectores",
      "Pontes para outros sistemas",
      "Ligar o acervo a fontes externas (chats, e-mails, documentos, CRMs) para receber ou enviar informações automaticamente.",
      "Muito conhecimento do time está em outros sistemas. Os conectores trazem essas informações para o acervo e, quando faz sentido, levam de volta.",
      [
        "Catálogo de conectores disponíveis",
        "Cadastro com credenciais e escopo",
        "Teste rápido antes de ativar",
        "Histórico de eventos trocados",
      ],
      [
        { numero: 1, acao: "Abra o catálogo e escolha um conector", esperado: "Formulário de configuração abre" },
        { numero: 2, acao: "Preencha, teste e ative", esperado: "Status fica verde; eventos começam a chegar" },
      ],
      [
        "Credenciais ficam criptografadas — só o administrador pode ver",
        "Conectores inativos não são removidos — ficam disponíveis para reativar",
      ],
    ),
    en: pt(
      "Connectors",
      "Bridges to other systems",
      "Link the archive to external sources (chats, email, docs, CRMs) to pull or push information automatically.",
      "A lot of team knowledge lives in other systems. Connectors bring it into the archive and, when it makes sense, push back.",
      [
        "Catalogue of available connectors",
        "Setup with credentials and scope",
        "Quick test before enabling",
        "Exchanged-events history",
      ],
      [
        { numero: 1, acao: "Open the catalogue and pick a connector", esperado: "Setup form opens" },
        { numero: 2, acao: "Fill in, test and enable", esperado: "Status turns green; events start arriving" },
      ],
      [
        "Credentials are encrypted — only admins can see",
        "Inactive connectors aren't removed — stay ready to reactivate",
      ],
    ),
  },

  "/app/notes": {
    ptBR: pt(
      "Notas",
      "Seus rascunhos pessoais",
      "Um espaço para anotações rápidas que ainda não merecem virar memória formal. Organize ideias enquanto amadurecem.",
      "Nem toda ideia está pronta para virar memória compartilhada. Aqui você cuida das notas pessoais sem poluir o acervo do time.",
      [
        "Criar, editar e remover notas",
        "Marcar com tags próprias",
        "Promover uma nota para memória quando estiver madura",
      ],
      [
        { numero: 1, acao: "Crie uma nota com ideia em desenvolvimento", esperado: "Nota aparece na lista pessoal" },
        { numero: 2, acao: "Amadureça-a com edições", esperado: "Histórico local preservado" },
        { numero: 3, acao: "Promova para memória do tenant", esperado: "Vira parte do acervo compartilhado" },
      ],
    ),
    en: pt(
      "Notes",
      "Your personal drafts",
      "A space for quick notes not yet worth being formal memories. Organise ideas while they ripen.",
      "Not every idea is ready to be shared. Here you keep personal notes without cluttering the team archive.",
      [
        "Create, edit and remove notes",
        "Tag with your own labels",
        "Promote a note to memory once it's ripe",
      ],
      [
        { numero: 1, acao: "Create a note with an evolving idea", esperado: "Note appears in your personal list" },
        { numero: 2, acao: "Refine it over edits", esperado: "Local history preserved" },
        { numero: 3, acao: "Promote it to a tenant memory", esperado: "Joins the shared archive" },
      ],
    ),
  },

  "/app/tasks": {
    ptBR: pt(
      "Tarefas",
      "Rotinas que rodam sozinhas",
      "Ver e configurar as tarefas automáticas do sistema: limpeza, relatórios, sincronizações recorrentes.",
      "Muita manutenção é silenciosa — acontece às 3 da manhã. Esta tela traz visibilidade: o que roda, com que frequência, quando rodou da última vez, se deu certo.",
      [
        "Lista das tarefas com status e próxima execução",
        "Detalhes da última execução (sucesso, duração, itens afetados)",
        "Pausar ou disparar manualmente uma tarefa",
      ],
      [
        { numero: 1, acao: "Observe a tarefa 'Limpeza semanal'", esperado: "Indica próxima execução" },
        { numero: 2, acao: "Dispare agora", esperado: "Relatório imediato aparece com o resultado" },
      ],
      [
        "Tarefas falhas consecutivas geram alerta — verifique logs antes de pausar",
        "Rodar manualmente não altera a programação automática",
      ],
    ),
    en: pt(
      "Tasks",
      "Routines that run on their own",
      "See and tune automatic tasks: cleanup, reports, recurring syncs.",
      "Lots of maintenance is silent — it runs at 3am. This screen gives visibility: what runs, how often, last run, success/failure.",
      [
        "List of tasks with status and next run",
        "Last-run detail (success, duration, items affected)",
        "Pause or trigger a task manually",
      ],
      [
        { numero: 1, acao: "Check the 'Weekly cleanup' task", esperado: "Shows next run" },
        { numero: 2, acao: "Trigger now", esperado: "Immediate report with the outcome" },
      ],
      [
        "Consecutive failures raise an alert — review logs before pausing",
        "Manual runs don't change the automatic schedule",
      ],
    ),
  },
};

/**
 * Look up help content for a given route, tolerant of trailing paths.
 * E.g. `/app/memories/123` falls back to `/app/memories`.
 */
export function getHelpContent(route: string): HelpEntry | undefined {
  if (helpContent[route]) return helpContent[route];
  const parts = route.split("/");
  while (parts.length > 1) {
    parts.pop();
    const candidate = parts.join("/");
    if (helpContent[candidate]) return helpContent[candidate];
  }
  return undefined;
}
