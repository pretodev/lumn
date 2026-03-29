# lumn — Documento de Visão do Produto

> **Status:** Draft v0.2
> **Última atualização:** Março 2025
> **Audiência:** Colaboradores, early adopters, investidores técnicos

---

## Índice

1. [O problema](#1-o-problema)
2. [A proposta](#2-a-proposta)
3. [Princípios de design](#3-princípios-de-design)
4. [Público-alvo](#4-público-alvo)
5. [O que é lumn](#5-o-que-é-lumn)
6. [A linguagem de workflows](#6-a-linguagem-de-workflows)
7. [Ecossistema de plugins](#7-ecossistema-de-plugins)
8. [Interface visual](#8-interface-visual)

- **Sem testabilidade.** Nessas ferramentas, não existe um comando simples como `lumn run order_cancel` — a única forma de testar é executar manualmente no ambiente.

10. [Data Tables](#10-data-tables)
11. [Triggers](#11-triggers)
12. [CLI e daemon](#12-cli-e-daemon)
13. [Integração com IA via MCP](#13-integração-com-ia-via-mcp)
14. [Deploy e operação](#14-deploy-e-operação)
15. [Arquitetura e design técnico](#15-arquitetura-e-design-técnico)
16. [Comparativo com alternativas](#16-comparativo-com-alternativas)
17. [Roadmap de produto](#17-roadmap-de-produto)
18. [Posicionamento open source](#18-posicionamento-open-source)

---

## 1. O problema

Automação de processos de negócio é uma necessidade universal. Toda equipe de engenharia acaba construindo — ou adotando — algum sistema que conecta APIs, processa dados, dispara ações e reage a eventos. O mercado atual oferece duas escolhas, e ambas apresentam limitações sérias.

### Ferramentas visuais (n8n, Make, Zapier)

Ferramentas no-code e low-code democratizaram a automação. Qualquer pessoa consegue conectar um webhook ao Slack em minutos. Mas essa acessibilidade tem um custo que cresce junto com a complexidade:

- **Lógica real não cabe no canvas.** Um fluxo com condicionais aninhadas, transformações de dados e tratamento de erros vira um labirinto visual ilegível.
- **Versionamento é cidadão de segunda classe.** Workflows vivem em banco de dados, não em arquivos de texto. Diff, code review e rollback são dores constantes.
- **Sem testabilidade.** Não existe `lumn run order_cancel` — a única forma de testar é executar manualmente no ambiente.
- **Reuso limitado.** Copiar e colar subfluxos entre projetos é a norma. Abstrações reutilizáveis não existem como conceito de primeira classe.
- **Lock-in de plataforma.** Migrar de n8n para outra ferramenta significa reescrever tudo do zero.

### Ferramentas de engenharia (Airflow, Prefect, Temporal)

O outro extremo são orquestradores voltados para engenharia de dados e microsserviços. São poderosos, mas carregam um peso operacional que os torna inacessíveis para a maioria dos casos de uso:

- **Curva de aprendizado íngreme.** Airflow exige entender DAGs em Python, operadores, XComs, conexões e um modelo de deployment complexo.
- **Infraestrutura pesada.** Temporal requer um cluster dedicado. Airflow precisa de scheduler, worker e webserver separados. Prefect tem seu próprio servidor de orquestração.
- **Over-engineering para casos simples.** Processar e-mails de cancelamento não deveria exigir o mesmo setup que um pipeline de dados de petabytes.
- **Sem foco em integrações de negócio.** Essas ferramentas são otimizadas para dados, não para fluxos que consomem APIs externas, enviam e-mails e manipulam documentos.

### O gap

Existe um espaço vazio entre "fácil mas limitado" e "poderoso mas complexo". Um desenvolvedor experiente que quer automatizar um processo real não precisa de um canvas drag-and-drop nem de um cluster Kubernetes. Precisa de uma ferramenta que respeite seu ofício: código versionável, testável, reutilizável, com a expressividade de uma linguagem de programação real.

---

## 2. A proposta

**lumn** é um orquestrador de workflows para desenvolvedores que valorizam simplicidade operacional e expressividade programática.

Os workflows são escritos em **Lua** — uma linguagem leve, embarcável e com sintaxe clara — através de uma DSL projetada para descrever fluxos de dados de forma declarativa. O resultado é um arquivo Lua comum, versionável com Git, testável com um único comando e executável em qualquer ambiente com um único binário.

A plataforma entrega tudo que um desenvolvedor precisa para colocar automações em produção: um daemon local, uma interface visual para observação e depuração, um gerenciador de credenciais seguro, um sistema de triggers variados, armazenamento integrado e um ecossistema de plugins compartilháveis.

O objetivo não é substituir engenharia de dados complexa. O objetivo é ser a ferramenta que um time pequeno de engenheiros escolhe para automatizar os processos de negócio da sua empresa — e que continua sendo a escolha certa à medida que esses processos crescem em número e complexidade.

---

## 3. Princípios de design

Estes princípios guiam cada decisão de produto e cada linha de código. Quando dois requisitos entram em conflito, eles determinam qual prevalece.

**Código é o artefato primário.**
Workflows são arquivos Lua comuns, organizados em pastas, versionados com Git. Eles passam por code review, têm histórico de commits e podem ser testados em CI. Nunca existe uma fonte de verdade invisível escondida em banco de dados de uma plataforma externa.

**Um binário, zero dependências.**
Instalar o `lumn` é baixar um executável. Não existe runtime separado, não existe banco de dados externo obrigatório, não existe servidor de licença. O desenvolvedor executa `lumn daemon start` e está pronto para começar.

**Complexidade progressiva.**
Um workflow simples deve ser simples de escrever. A complexidade da ferramenta só aparece quando o problema exige. Um workflow de três steps não deve forçar o desenvolvedor a entender conceitos avançados que só importam em workflows de cinquenta steps.

**Observabilidade como feature de primeira classe.**
O desenvolvedor precisa saber o que está acontecendo. Cada execução, cada step, cada item que passou ou foi descartado é registrado e visualizável. Depurar um workflow não pode exigir navegar por logs de servidor em formato raw.

**O ecossistema pertence à comunidade.**
Plugins são pacotes Lua ou Go publicados em repositórios Git. Qualquer pessoa pode criar, publicar e compartilhar. O processo de adoção é tão simples quanto `lumn plugin add pretodev/outlook`. Cada plugin que precisa de autenticação fornece seu próprio fluxo de setup — o desenvolvedor nunca precisa descobrir sozinho como configurar um OAuth.

**Segurança sem atrito.**
Credenciais são armazenadas com criptografia de ponta e nunca aparecem em logs. O modelo de segurança é seguro por padrão — o desenvolvedor não precisa fazer nada especial para evitar vazar um segredo.

---

## 4. Público-alvo

### Desenvolvedor individual / freelancer

Automatiza processos para clientes ou projetos pessoais. Precisa de algo que instale rápido, funcione localmente e possa subir num VPS barato quando precisar rodar continuamente. Não quer gerenciar infraestrutura; quer focar na lógica de negócio.

### Time de engenharia de pequeno porte (2–15 pessoas)

Tem um monorepo com integrações e automações crescendo organicamente. Hoje usa scripts shell, cron jobs e n8n misturados. Quer consolidar tudo em uma ferramenta com versionamento real, observabilidade decente e um modelo de desenvolvimento que qualquer engenheiro do time consiga entender e manter.

### Engenheiro de operações / SRE

Precisa de automações confiáveis para tarefas operacionais: sincronização de dados, alertas com ação automática, onboarding de usuários, relatórios periódicos. Valoriza a capacidade de inspecionar execuções passadas e entender exatamente o que aconteceu quando algo deu errado.

### Desenvolvedor que cria integrações para clientes

Constrói automações customizadas como produto ou serviço. Precisa de uma ferramenta que possa empacotar e entregar como artefato — um container Docker com todos os workflows e dependências — sem expor acesso ao ambiente de desenvolvimento.

---

## 5. O que é lumn

lumn é composto por cinco componentes que trabalham juntos:

```
┌──────────────────────────────────────────────────────┐
│                   lumn platform                      │
│                                                      │
│  ┌──────────┐  ┌──────────┐  ┌────────────────────┐ │
│  │  CLI     │  │  Web UI  │  │   MCP server       │ │
│  │ (lumn)   │  │          │  │   (AI assistant)   │ │
│  └────┬─────┘  └────┬─────┘  └──────────┬─────────┘ │
│       │             │                   │            │
│  ┌────▼─────────────▼───────────────────▼──────────┐ │
│  │               Daemon (lumnd)                    │ │
│  │   Trigger system · Engine · Executor            │ │
│  └────────────────────┬────────────────────────────┘ │
│                       │                              │
│  ┌────────────────────▼────────────────────────────┐ │
│  │              Storage layer                      │ │
│  │  Credentials vault · Execution logs · Data tables │
│  └─────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

**CLI (`lumn`)** é o ponto de entrada do desenvolvedor. Cria projetos, executa workflows, gerencia plugins, credenciais e o daemon.

**Daemon (`lumnd`)** é o processo que fica em background, responsável por monitorar triggers, executar workflows agendados e manter o estado de execução. É o coração da plataforma em produção.

**Web UI** é a interface visual servida pelo daemon. Mostra os workflows ativos, o estado de cada execução em tempo real, histórico de runs e o inspetor de steps.

**MCP server** expõe as capacidades do lumn como ferramentas para LLMs. Um desenvolvedor pode descrever um workflow em linguagem natural para um assistente de IA que tem o MCP server conectado e receber o arquivo Lua pronto para uso.

**Storage layer** engloba o vault criptografado de credenciais, o banco de logs de execução e as Data Tables — tabelas SQLite gerenciadas pela plataforma e acessíveis dentro dos workflows.

---

## 6. A linguagem de workflows

### Por que Lua

A escolha de Lua como linguagem de definição de workflows não é acidental. Lua foi projetada especificamente para ser embarcada em aplicações maiores — é a linguagem de script do Redis, do Nginx, do Neovim, de milhares de jogos. Ela tem exatamente as propriedades certas para uma DSL de workflows:

- **Sintaxe simples e legível.** Um desenvolvedor sem experiência prévia com Lua consegue ler e entender um workflow em minutos.
- **Tables como estrutura universal.** Toda configuração em Lua é uma table. Não existe parsing de YAML ou JSON no meio do caminho.
- **Funções como valores de primeira classe.** Handlers de erro, transformações de dados e condições de branch são funções Lua inline, sem template engine.
- **Embarcável com segurança.** É trivial restringir quais funções da biblioteca padrão ficam disponíveis para o código do usuário.

### Estrutura de um workflow

Um workflow lumn é um **arquivo Lua** que retorna uma table com a definição do fluxo. O nome padrão é `lumn.lua` — análogo ao `Dockerfile` do Docker. Quando qualquer comando lumn é executado sem especificação explícita, o runtime procura `lumn.lua` automaticamente no diretório atual.

Para workflows mais complexos — com templates, módulos auxiliares e assets — a estrutura pode ser uma **pasta**. Nesse caso, o entrypoint é `init.lua`, que tem prioridade sobre `lumn.lua` quando ambos existem.

#### Regras de resolução de entrypoint

| Situação                          | Entrypoint procurado | Prioridade |
| --------------------------------- | -------------------- | ---------- |
| Diretório atual, sem argumento    | `./lumn.lua`         | —          |
| Pasta especificada                | `pasta/init.lua`     | 1º         |
| Pasta especificada (sem init.lua) | `pasta/lumn.lua`     | 2º         |
| Arquivo especificado com `-f`     | o arquivo informado  | exato      |

```
meu-projeto/
│
├── lumn.lua                   ← workflow simples (entrypoint direto)
│
├── order_cancel/
│   ├── init.lua               ← entrypoint do workflow complexo
│   ├── templates/
│   │   ├── aprovado.html
│   │   └── negado.html
│   └── utils.lua              ← módulo auxiliar local
│
├── customer_sync/
│   └── lumn.lua               ← também válido como entrypoint de pasta
│
└── lumn.lock                  ← versões fixas dos plugins
```

#### Formato do arquivo de workflow

O arquivo retorna uma table com a definição do fluxo. Não existem metadados de identidade (`id`, `name`, `version`) no arquivo — esses são definidos em tempo de `lumn start`, pelo operador. O arquivo é código portável e reutilizável; a identidade é responsabilidade do runtime que o executa.

```lua
-- order_cancel/init.lua

local outlook = lumn.plugins.outlook { key = "outlook.cancelamentos" }
-- ... demais componentes ...

return {
  triggers = {
    lumn.triggers.scheduler { interval = "15m" },
  },

  flow = {
    call { ... },
    pipe { ... },
    -- ...
  },

  on_error = {
    default = "skip_item",
  },
}
```

A separação é intencional: o mesmo arquivo pode ser registrado no daemon com nomes e versões diferentes em ambientes distintos — staging, produção, canary — sem nenhuma alteração no código.

### O global `lumn`

As ferramentas, integrações e primitivos da plataforma são acessados através do global `lumn`, injetado pelo runtime no momento da execução. Não existe `require` para recursos da plataforma — `lumn` é o namespace único de tudo que o engine oferece.

O global `lumn` organiza seus recursos em dois grupos:

**Utilitários nativos do runtime** — sempre disponíveis, sem instalação de plugin:

```lua
lumn.http.client { ... }       -- cliente HTTP genérico
lumn.http.post { ... }         -- shorthand para POST
lumn.ai.agent { ... }          -- agente IA
lumn.ai.model.azure_openai { } -- modelo OpenAI via Azure
lumn.ai.structured_parser { }  -- parser de output estruturado
lumn.auth.bearer("key")        -- resolve token do estado global
lumn.date.now()                -- data/hora atual
lumn.date.add(date, days)      -- aritmética de datas
lumn.env("NOME")               -- variável de ambiente
lumn.secret("NOME")            -- credencial do vault
lumn.get("key")                -- lê estado global do workflow
lumn.set("key", value)         -- grava estado global do workflow
lumn.triggers.scheduler { }    -- trigger por intervalo/cron
lumn.triggers.webhook { }      -- trigger por HTTP
lumn.triggers.file_watcher { } -- trigger por evento de arquivo
```

**Plugins instalados** — acessados via `lumn.plugins.<nome>`:

```lua
lumn.plugins.outlook { ... }          -- Microsoft Outlook / Graph API
lumn.plugins.sendgrid.send { ... }    -- SendGrid e-mail
lumn.plugins.gdrive { ... }           -- Google Drive
lumn.plugins.slack.message { ... }    -- Slack
lumn.plugins.aws.s3 { ... }           -- Amazon S3
```

A distinção é clara: `require` carrega arquivos Lua do disco (código local do projeto); `lumn.*` acessa o runtime; `lumn.plugins.*` acessa plugins instalados via `lumn plugin add`.

### Modelo de pipeline

Um workflow opera sobre uma **lista de itens** que flui por uma sequência de primitivos. Cada primitivo recebe a lista, faz algo com ela, e passa o resultado para o próximo.

```
[emails] → call → tap → pipe → distinct → filter → once → pipe → pipe → set → branch → [sent]
```

Esse modelo é intuitivo para qualquer desenvolvedor que já usou `Array.map/filter` em JavaScript ou pipes em Unix. Quando a lista de itens fica vazia em qualquer ponto do fluxo — por um `filter` sem resultados, por uma fonte sem dados, ou por erros que descartaram todos os itens — o workflow encerra naturalmente com status `"empty"`. Não existe primitivo especial para isso; é o comportamento padrão do runtime.

### Primitivos da DSL

Todos os primitivos usam **sintaxe de table** — `primitivo { chave = valor }`. Não existe mistura de estilos: tudo é uma table com chaves nomeadas, o que torna o código uniforme independentemente do primitivo.

| Primitivo  | Contrato                                                                                                                        | Muta o item?   |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------- | -------------- |
| `call`     | Cria a lista de itens a partir de uma fonte externa. `on_data(result)` retorna a forma inicial do item.                         | — (cria itens) |
| `tap`      | Efeito colateral puro. O callable recebe o item diretamente; o resultado é descartado.                                          | Nunca          |
| `pipe`     | Chama um callable por item e mergeia o resultado. `on_data(item, result)` retorna o item atualizado.                            | Sim            |
| `set`      | Transformação Lua pura, sem chamada externa. `to(item)` calcula valores derivados e retorna o item.                             | Sim            |
| `filter`   | Remove itens onde `condition(item)` retorna falso.                                                                              | Não            |
| `distinct` | Remove duplicatas. `by(item)` retorna a chave de deduplicação; duplicatas são descartadas silenciosamente.                      | Não            |
| `once`     | Executa um callable uma única vez para todo o lote (barreira). `on_data(result)` usa `lumn.set` para escrever no estado global. | Não            |
| `branch`   | Roteia cada item para um sub-pipeline baseado em `condition(item)`.                                                             | Condicional    |
| `parallel` | Executa sub-pipelines concorrentemente e aguarda todos convergirem.                                                             | Depende        |

### A separação entre `call`, `pipe` e `set`

Estes três primitivos cobrem todos os casos de transformação e têm contratos intencionalmente distintos. Em todos eles, o callable recebe o `item` diretamente — não existe `select` no primitivo. Cada callable declara em sua própria config como quer usar o item (via `query`, `message`, `select` ou qualquer campo que o plugin definir).

**`call`** é o primitivo de _fonte_. Cria a lista de itens do zero. `on_data` recebe o resultado bruto da fonte e retorna a forma inicial do item — não existe `item` anterior porque este é o primeiro step.

```lua
call {
  exec    = lumn.plugins.outlook.message.list { folder = "Inbox", unread = true },
  on_data = function(result)
    return {
      email_id    = result.id,
      received_at = result.received_datetime,
      email_body  = result.body.content,
    }
  end,
}
```

**`tap`** é o primitivo de _efeito colateral_. O callable recebe o item e age sobre ele (mover um arquivo, disparar um evento, logar). O resultado é sempre descartado; o item passa inalterado.

```lua
tap {
  exec = lumn.plugins.outlook.message.move {
    folder = "Processando",
    select = function(item) return item.email_id end,  -- config interna do callable
  },
},
```

**`pipe`** é o primitivo de _transformação com I/O externo_. O callable recebe o item via sua própria config — `query`, `message`, `path` ou o que o plugin definir. `on_data` mergeia o resultado de volta no item.

```lua
pipe {
  exec    = agent_extract {
    message = function(item) return item.email_body end,  -- config interna do callable
  },
  on_data = function(item, result)
    item.pedido_id  = result.pedidoId
    item.client_cpf = result.cpf
    return item
  end,
}
```

**`set`** é o primitivo de _transformação pura_. Sem callable externo — apenas Lua. Usa `lumn.get` para acessar estado global quando necessário.

```lua
set {
  to = function(item)
    local days = (item.client_level == "diamond") and 14 or 7
    item.is_within_period = lumn.date.now() <= lumn.date.add(item.order_date, days)
    item.grace_days       = days
    return item
  end,
}
```

### Estado por item vs. estado global

Uma distinção central no modelo é a separação entre dois tipos de estado:

**`item`** é o objeto que carrega os dados de um elemento específico da pipeline — um e-mail, um pedido, um registro. Cada item tem sua própria cópia independente e é passado como argumento dos callbacks. Mutações em um item nunca afetam outros.

**Estado global** é compartilhado entre todos os items de uma execução e gerenciado através de duas funções do runtime:

- `lumn.set("chave", valor)` — grava um valor no estado global do workflow
- `lumn.get("chave")` — lê um valor do estado global

Nenhum callback recebe o contexto como argumento. O acesso ao estado global é sempre explícito, via `lumn.get` e `lumn.set`, o que torna imediatamente visível no código quando uma função depende de estado compartilhado — sem necessidade de inspecionar a assinatura da função.

O access token OAuth é o exemplo canônico: gravado uma única vez pelo `once`, lido por todos os `pipe` e `set` subsequentes.

```lua
-- once grava no estado global via lumn.set
once {
  exec    = get_access_token,
  on_data = function(result)
    lumn.set("access_token", result.data.access_token)
  end,
}

-- pipe: o callable recebe o item via sua própria config
-- lumn.auth.bearer resolve lumn.get("access_token") internamente
pipe {
  exec = sap.get {
    path  = "/Customer/CustomerList",
    auth  = lumn.auth.bearer("access_token"),
    query = function(item)
      return { ["$filter"] = "CPF eq '" .. item.client_cpf .. "'" }
    end,
  },
  on_data = function(item, result)
    item.client_level = result.data.customerLevel
    return item
  end,
}

-- set lê o estado global diretamente quando necessário
set {
  to = function(item)
    local tenant = lumn.get("tenant_id")   -- leitura explícita de estado global
    item.url = "https://" .. tenant .. ".api.com/orders/" .. item.order_id
    return item
  end,
}
```

### Variáveis de ambiente

Variáveis de ambiente e secrets são acessados via `lumn.env` e `lumn.secret` — nunca via `os.getenv` ou qualquer outra função do sistema que o sandbox bloqueia:

```lua
-- Variável de ambiente (string, lida do .env ou do ambiente do processo)
lumn.env("SAP_BASE_URL")

-- Secret (lido do vault criptografado; valor nunca aparece em logs)
lumn.secret("SAP_CLIENT_SECRET")
```

A distinção entre `lumn.env` e `lumn.secret` é intencional: `env` é para configuração não-sensível que pode aparecer em logs e no output de `lumn status`; `secret` é para credenciais que o engine garante nunca serializar.

### Exemplo comentado

O workflow completo de cancelamento de pedidos mostra todos os primitivos em contexto real:

```lua
-- order_cancel/init.lua

-- ── Declaração de componentes ───────────────────────────────────────────────
-- Plugins acessados via lumn.plugins.*
-- Utilitários nativos do runtime acessados via lumn.http, lumn.ai, etc.

local outlook = lumn.plugins.outlook {
  key = "outlook.cancelamentos",
}

local agent_extract = lumn.ai.agent {
  system_message = [[
    Extraia os dados do formulário de cancelamento.
    Retorne SOMENTE JSON válido, sem markdown.
  ]],
  model = lumn.ai.model.azure_openai {
    model_name = "gpt-4o",
    key        = "azure.openai",
  },
  output_parser = lumn.ai.structured_parser {
    fields = {
      { name = "pedidoId", type = "string", required = true  },
      { name = "cpf",      type = "cpf",    required = true  },
      { name = "name",     type = "string", required = false },
      { name = "email",    type = "email",  required = false },
      { name = "phone",    type = "string", required = false },
      { name = "reason",   type = "string", required = false },
    },
    on_invalid = "skip",
  },
}

local get_access_token = lumn.http.post {
  url          = "https://login.microsoftonline.com/token",
  content_type = "application/x-www-form-urlencoded",
  data = {
    grant_type    = "client_credentials",
    resource      = lumn.env("SAP_RESOURCE"),
    client_id     = lumn.env("SAP_CLIENT_ID"),
    client_secret = lumn.secret("SAP_CLIENT_SECRET"),
  },
}

local sap = lumn.http.client {
  base_url = lumn.env("SAP_BASE_URL"),
  headers  = {
    ["Ocp-Apim-Subscription-Key"] = lumn.env("OCP_KEY"),
  },
}

local send_mail = lumn.plugins.sendgrid.send {
  sender_email = "no-reply@bemoldigital.com.br",
  sender_name  = "Bemol Digital",
  subject      = "Sua solicitação de cancelamento",
  mime_type    = lumn.plugins.sendgrid.html_mime_type,
}

-- ── Definição do workflow ────────────────────────────────────────────────────
-- Sem id, name ou version — identidade definida em lumn start, não no arquivo.

return {
  triggers = {
    lumn.triggers.scheduler { interval = "15m" },
  },

  flow = {

    -- 1. Busca e-mails não lidos e cria a lista de itens
    call {
      exec    = outlook.message.list { folder = "Inbox", unread = true },
      on_data = function(result)
        return {
          email_id    = result.id,
          received_at = result.received_datetime,
          email_body  = result.body.content,
        }
      end,
    },

    -- 2. Arquiva o e-mail (efeito colateral; item não muda)
    -- select é config interna do callable, não do primitivo tap
    tap {
      exec = outlook.message.move {
        folder = "Processando",
        select = function(item) return item.email_id end,
      },
    },

    -- 3. Extrai dados estruturados via IA
    -- message é config interna do agent_extract, não do primitivo pipe
    pipe {
      exec    = agent_extract {
        message = function(item) return item.email_body end,
      },
      on_data = function(item, result)
        item.pedido_id    = result.pedidoId
        item.client_name  = result.name
        item.client_cpf   = result.cpf
        item.client_email = result.email
        return item
      end,
    },

    -- 4. Remove duplicatas do mesmo lote
    distinct { by = function(item) return item.pedido_id end },

    -- 5. Descarta itens inválidos
    filter {
      condition = function(item)
        return item.client_cpf ~= nil
           and #item.client_cpf == 11
           and item.pedido_id   ~= nil
      end,
    },

    -- 6. Obtém token OAuth — uma vez para todo o lote
    once {
      exec    = get_access_token,
      on_data = function(result)
        lumn.set("access_token", result.data.access_token)
      end,
    },

    -- 7. Consulta nível do cliente
    -- query é config interna do callable sap.get
    pipe {
      exec    = sap.get {
        path  = "/Customer/CustomerList",
        auth  = lumn.auth.bearer("access_token"),
        query = function(item)
          return {
            ["$filter"] = "CPF eq '" .. item.client_cpf .. "'",
            ["$expand"] = "Level",
          }
        end,
      },
      on_data = function(item, result)
        item.client_id    = result.data.customerId
        item.client_name  = result.data.customerName
        item.client_level = result.data.customerLevel
        return item
      end,
    },

    -- 8. Consulta pedido e nota fiscal
    pipe {
      exec    = sap.get {
        path  = "/order/Order",
        auth  = lumn.auth.bearer("access_token"),
        query = function(item)
          return {
            ["$filter"] = "Customer eq '" .. item.client_id .. "'",
            ["$expand"] = "NotaFiscalDetails",
          }
        end,
      },
      on_data = function(item, result)
        item.order_number = result.data.OriginOrderNumber
        item.order_date   = result.data.OrderDate
        item.order_type   = (result.data.StoreID == "102") and "ONLINE" or "FISICO"
        return item
      end,
    },

    -- 9. Calcula prazo — lógica pura, sem I/O
    set {
      to = function(item)
        local days = (item.client_level == "diamond") and 14 or 7
        item.is_within_period = lumn.date.now() <= lumn.date.add(item.order_date, days)
        item.grace_days       = days
        return item
      end,
    },

    -- 10. Envia e-mail conforme prazo
    branch {
      condition = function(item) return item.is_within_period end,

      on_true = tap {
        exec = send_mail {
          to   = function(item) return item.client_email end,
          body = function(item)
            return table.concat({
              "nome    = " .. item.client_name,
              "pedido  = " .. item.order_number,
              "prazo   = " .. item.grace_days .. " dias",
            }, "\n")
          end,
        },
      },

      on_false = tap {
        exec = send_mail {
          to   = function(item) return item.client_email end,
          body = function(item)
            return table.concat({
              "nome   = " .. item.client_name,
              "pedido = " .. item.order_number,
            }, "\n")
          end,
        },
      },
    },

  },

  on_error = {
    default          = "skip_item",
    agent_extract    = "manual_review",
    get_access_token = "fail",
  },
}
```

---

## 7. Ecossistema de plugins

### Modelo de distribuição

Plugins no lumn seguem o mesmo modelo do Go Modules e dos plugins do lazy.nvim: um repositório Git público é a unidade de distribuição. A referência de um plugin é uma forma compacta do caminho do repositório:

```sh
# Formato curto: usuario/plugin
lumn plugin add pretodev/outlook

# Com caminho interno ao repositório
lumn plugin add pretodev/plugins/outlook

# Com versão específica
lumn plugin add pretodev/outlook@v2.1.0

# Com host Git explícito (padrão é github.com)
lumn plugin add gitlab.com/usuario/outlook
```

O `lumn.lock` no projeto registra as versões exatas e hashes de todos os plugins instalados, garantindo que dois desenvolvedores com o mesmo `lumn.lock` tenham exatamente o mesmo comportamento — independentemente de atualizações publicadas no repositório do plugin.

### Setup de credenciais por plugin

Uma das decisões mais importantes do ecossistema de plugins do lumn é que **cada plugin é responsável por seu próprio fluxo de autenticação**. O core da plataforma não tenta abstrair OAuth, API keys, tokens, certificados e outros mecanismos em uma interface única — isso seria impossível de fazer bem.

Em vez disso, cada plugin pode declarar um ou mais **comandos de credencial**. Quando o usuário executa `lumn credential add <provider>`, o plugin recebe o controle e conduz o fluxo adequado:

```sh
# Plugin pretodev/outlook: abre browser para OAuth Microsoft
lumn credential add outlook

# Plugin pretodev/gdrive: OAuth Google com seleção de escopos
lumn credential add gdrive

# Plugin acme/sap: formulário interativo para client_id, secret e tenant
lumn credential add sap

# Plugin acme/aws: suporte a perfis do ~/.aws/credentials ou input manual
lumn credential add aws
```

O resultado de cada fluxo é salvo no vault criptografado do lumn, sob o nome declarado pelo plugin. O workflow acessa via `lumn.secret("nome-da-credencial")` sem precisar saber como ela foi obtida.

Esse modelo tem três consequências importantes:

**Para o usuário:** setup guiado, sem precisar consultar documentação de APIs externas para descobrir quais parâmetros preencher.

**Para o autor do plugin:** liberdade total para implementar o fluxo correto — OAuth com PKCE, device flow, API key simples, certificado mTLS, o que for necessário.

**Para a segurança:** a credencial nunca transita por texto no terminal nem é armazenada em variáveis de ambiente visíveis — vai direto do fluxo de autenticação para o vault.

### Dois tipos de plugins

**Plugins Go** são a forma canônica para integrações que precisam de performance, acesso a SDKs existentes ou manipulação de dados binários. São processos separados que se comunicam com o engine via gRPC — isolamento real, sem acesso direto à memória do engine.

**Plugins Lua** são a forma mais simples para integrações sobre HTTP simples ou lógica utilitária. Um plugin Lua é uma pasta com um `init.lua` que exporta callables. Não requer compilação; funciona em qualquer plataforma que rode o lumn.

### Biblioteca padrão

O lumn vem com uma biblioteca padrão de plugins mantida pela equipe principal. Plugins instalados são acessados via `lumn.plugins.<nome>`; utilitários nativos do runtime (`lumn.http`, `lumn.ai`) não requerem instalação:

| Plugin          | Namespace               | Descrição                                             | Credential command                            |
| --------------- | ----------------------- | ----------------------------------------------------- | --------------------------------------------- |
| nativo          | `lumn.http.*`           | Cliente HTTP, POST, auth — embutido no runtime        | —                                             |
| nativo          | `lumn.ai.*`             | Agent, model, structured_parser — embutido no runtime | `lumn credential add openai` / `azure-openai` |
| `lumn/outlook`  | `lumn.plugins.outlook`  | E-mails via Microsoft Graph API                       | `lumn credential add outlook`                 |
| `lumn/gdrive`   | `lumn.plugins.gdrive`   | Arquivos via Google Drive API                         | `lumn credential add gdrive`                  |
| `lumn/sendgrid` | `lumn.plugins.sendgrid` | Envio de e-mail via SendGrid                          | `lumn credential add sendgrid`                |
| `lumn/smtp`     | `lumn.plugins.smtp`     | Envio via SMTP genérico                               | `lumn credential add smtp`                    |
| `lumn/slack`    | `lumn.plugins.slack`    | Mensagens para canais Slack                           | `lumn credential add slack`                   |
| `lumn/aws-s3`   | `lumn.plugins.aws.s3`   | Objetos no Amazon S3                                  | `lumn credential add aws`                     |
| `lumn/data`     | `lumn.plugins.data`     | Acesso às Data Tables integradas                      | —                                             |

### Criando um plugin

A interface pública de um plugin Go é mínima:

```go
type Plugin interface {
    Metadata()    PluginMetadata      // nome, versão, autor, descrição
    Primitives()  []PrimitiveSpec     // callables expostos ao Lua runtime
    Credentials() []CredentialSpec    // comandos de setup de credencial
    Teardown(ctx context.Context) error
}
```

`CredentialSpec` permite ao plugin declarar o nome do comando e fornecer um handler Go que conduz o fluxo de autenticação — pode abrir um browser para OAuth, apresentar um formulário interativo no terminal, ou qualquer outra coisa necessária.

---

## 8. Interface visual

A Web UI é servida diretamente pelo daemon — sem servidor separado, sem configuração. O desenvolvedor executa `lumn ui` e o browser abre automaticamente.

### Dashboard de workflows

A tela principal lista todos os workflows registrados no daemon. Para cada um, mostra:

- Status atual (ativo, pausado, sem trigger configurado)
- Próxima execução agendada (para schedulers)
- Resultado da última execução: sucesso, erro ou vazio (fluxo encerrou sem itens)
- Contagem de execuções nas últimas 24 horas

### Visualizador de DAG

Ao clicar em um workflow, o desenvolvedor vê o grafo de execução renderizado visualmente. Cada node representa um primitivo do `flow`. As arestas mostram a direção do fluxo de dados.

Durante uma execução ativa, o estado de cada node atualiza em tempo real via WebSocket:

- Cinza: aguardando
- Azul pulsando: executando
- Verde: concluído com sucesso
- Vermelho: erro
- Amarelo: item descartado (skip_item, filter, distinct)
- Roxo: manual_review

### Inspetor de steps

Clicar em qualquer node do DAG abre um painel lateral com:

- Quantos itens entraram no step
- Quantos saíram (e quantos foram descartados e por quê)
- O valor de `res` e `item` do último processamento (para `set` e `exec`)
- Duração média de execução por item
- Stack trace completo em caso de erro, com número de linha do `init.lua`

### Histórico de execuções

Uma timeline de todas as execuções passadas com:

- Timestamp de início e duração total
- Status: `success`, `error`, `empty`
- Número de itens processados em cada step
- Filtros por workflow, status e intervalo de datas
- Capacidade de inspecionar qualquer execução passada com o estado completo de cada step naquele run

### Trigger manager

Lista todos os triggers ativos com opção de pausar, ativar e editar. Para schedulers, mostra o horário calculado da próxima execução no fuso horário configurado. Para webhooks, mostra a URL gerada e o status da última requisição recebida.

### Credential manager

Interface visual para o vault de credenciais:

- Lista de todas as credenciais com nome, plugin de origem e data de criação
- Botão para executar o fluxo de renovação de uma credencial expirada
- Indicador de status (válida, expirando em breve, expirada)

---

## 9. Gerenciamento de credenciais

Credenciais são um dos pontos de falha mais comuns em sistemas de automação. Secrets aparecem em logs, são comitados acidentalmente no Git, ou ficam espalhados em variáveis de ambiente sem rastreamento.

O lumn trata credenciais como um recurso de primeira classe com garantias explícitas.

### Modelo de segurança

Todas as credenciais são armazenadas em um vault local criptografado com **AES-256-GCM**. A chave de criptografia é derivada de uma master password via **Argon2id** — o algoritmo recomendado pelo OWASP para key derivation.

As credenciais nunca aparecem em:

- Logs de execução (em nenhum nível de verbosidade)
- Output do terminal
- Serialização de items da pipeline
- Exportações de execução

No código Lua, `lumn.secret("NOME")` retorna um **objeto opaco** — não uma string. O valor real só é resolvido pelo engine no momento em que a credencial é necessária (por exemplo, quando o plugin `http` constrói o header de Authorization). Isso impede que um `print(item)` acidental exponha um token.

### Fluxos de setup guiado por plugin

O comando `lumn credential add <provider>` é gerenciado pelo plugin correspondente. O core da plataforma provê apenas a infraestrutura de armazenamento; o fluxo de obtenção é de responsabilidade de quem melhor conhece o sistema externo.

```sh
# OAuth com browser
lumn credential add outlook
# → Abrindo browser para autenticação Microsoft...
# → Permissões solicitadas: Mail.Read, Mail.Send
# → Aguardando callback... OK
# → Credencial 'outlook' salva no vault.

# API key simples
lumn credential add openai
# → OpenAI API Key: ****
# → Credencial 'openai' salva no vault.

# Credencial com múltiplos campos
lumn credential add sap
# → SAP Client ID: ****
# → SAP Client Secret: ****
# → SAP Tenant URL: https://...
# → Credencial 'sap' salva no vault.
```

### CLI de credenciais

```sh
# Listar todas (apenas nomes, nunca valores)
lumn credential list
# → outlook      (via pretodev/outlook  |  atualizado em 15/03/2025)
# → openai       (via lumn/openai       |  atualizado em 10/03/2025)
# → sap          (via acme/sap          |  atualizado em 01/03/2025)

# Renovar uma credencial expirada
lumn credential renew outlook

# Remover
lumn credential remove outlook

# Exportar vault portável (re-criptografado com nova senha)
lumn credential export --output prod-vault.enc

# Importar em outro ambiente
lumn credential import prod-vault.enc
```

---

## 10. Data Tables

Data Tables são tabelas SQLite gerenciadas pelo lumn e acessíveis dentro dos workflows via `lumn.data.table`. Elas resolvem o problema de estado persistente entre execuções — algo que variáveis de ambiente e arquivos em disco resolvem de forma frágil.

### Criando e usando uma Data Table

```lua
local pedidos = lumn.data.table {
  name   = "pedidos_processados",
  schema = {
    pedido_id     = { type = "string",   primary_key = true },
    status        = { type = "string"  },
    processado_em = { type = "datetime" },
  },
}

-- Filtra pedidos já processados em runs anteriores
filter(function(item)
  return pedidos.find(item.pedido_id) == nil
end),

-- Registra após processar
tap(pedidos.insert {
  select = function(item)
    return {
      pedido_id     = item.pedido_id,
      status        = "processado",
      processado_em = lumn.date.now(),
    }
  end,
}),
```

### Interface visual de Data Tables

A Web UI inclui um editor de Data Tables com visualização tabular dos registros, filtros e ordenação por qualquer coluna, inserção e edição manual de registros e export para CSV.

### Usos comuns

- **Deduplicação entre runs:** registrar IDs já processados para evitar reprocessamento em execuções futuras do mesmo workflow.
- **Fila de revisão manual:** items com `on_error = "manual_review"` são inseridos automaticamente numa Data Table com campo de status e observação para triagem humana.
- **Cache de dados externos:** resultados de APIs com dados que mudam raramente podem ser cacheados localmente para reduzir latência e custo de chamadas externas.
- **Auditoria:** registro de todas as ações executadas por um workflow para fins de compliance ou diagnóstico.

---

## 11. Triggers

Triggers são o mecanismo pelo qual workflows são iniciados. O lumn suporta múltiplos triggers por workflow — um mesmo workflow pode ser iniciado por um scheduler e por um webhook simultaneamente.

### Scheduler

O trigger mais comum. Suporta intervalo simples e cron expression:

```lua
-- Por intervalo
lumn.triggers.scheduler { interval = "15m" },

-- Por intervalo mais explícito
lumn.triggers.scheduler { interval = "1h" },

-- Por cron expression com fuso horário
lumn.triggers.scheduler {
  cron     = "0 9 * * MON-FRI",
  timezone = "America/Manaus",
},
```

### Webhook

Gera um endpoint HTTP único para o workflow. Qualquer sistema externo pode disparar o workflow com uma requisição HTTP:

```lua
lumn.triggers.webhook {
  path   = "/hooks/novo-pedido",
  method = "POST",
  secret = lumn.secret("WEBHOOK_SECRET"),  -- validação HMAC-SHA256
},
```

O daemon expõe os webhooks num servidor HTTP local. A URL gerada é exibida na Web UI e via `lumn status`.

### File watcher

Monitora um diretório ou arquivo e dispara o workflow quando ocorre uma mudança:

```lua
lumn.triggers.file_watcher {
  path    = "/data/importacoes",
  pattern = "*.csv",
  event   = "create",  -- "create" | "modify" | "delete" | "any"
},
```

### Roadmap de triggers futuros

O sistema de triggers é extensível via plugins:

- `triggers.email` — dispara ao receber e-mail que atende a um filtro
- `triggers.database` — dispara ao detectar mudança em uma tabela (polling ou CDC)
- `triggers.queue` — consome mensagens de SQS, RabbitMQ ou Redis Pub/Sub
- `triggers.git` — dispara em push ou pull request via webhooks do GitHub/GitLab

---

## 12. CLI e daemon

### A CLI (`lumn`)

A CLI é o ponto de entrada para tudo. Ela se comunica com o daemon via socket Unix quando este está rodando, e opera em modo standalone para comandos que não precisam do daemon (como `run` e `validate`).

#### Resolução de arquivo

Assim como o Docker procura um `Dockerfile` automaticamente, o lumn procura um `lumn.lua` no diretório atual quando nenhum argumento é fornecido. Para especificar explicitamente um arquivo ou pasta, use a flag `-f`:

```sh
lumn run                        # procura ./lumn.lua automaticamente
lumn run -f cancelamento.lua    # arquivo explícito
lumn run -f order_cancel        # pasta: usa order_cancel/init.lua (ou lumn.lua)
lumn run order_cancel           # sem -f: 1º daemon por id/name, 2º pasta local, 3º arquivo .lua
```

Quando o argumento não usa `-f`, o daemon tem prioridade — se existir uma instância registrada com aquele id ou nome, ela é usada. Sem match no daemon, o runtime procura uma pasta com esse nome, e em seguida um arquivo `.lua`. A prioridade de resolução para pastas é sempre `init.lua` → `lumn.lua`.

#### Referência de comandos

```
── Execução e desenvolvimento ──────────────────────────────────────────────────

lumn run                          Executa lumn.lua do diretório atual (uma vez,
                                  sem daemon, logs no terminal — modo dev)
lumn run <id|name>                Prioridade: 1º daemon (instância registrada),
                                  2º pasta local, 3º arquivo .lua.
                                  Com daemon: executa fora do trigger, útil para teste manual
lumn run -f <arquivo|pasta>       Força execução de arquivo/pasta local (ignora daemon)

lumn validate                     Valida sintaxe Lua e DAG sem executar
lumn validate -f <arquivo|pasta>  Valida arquivo/pasta específico

── Ciclo de vida no daemon ──────────────────────────────────────────────────────

lumn start [name]                 Registra lumn.lua no daemon
                                  name: nome do workflow (padrão: nome da pasta atual)

lumn start [name] -f <alvo>       Registra arquivo ou pasta específica
                                  alvo pode ser: arquivo.lua ou pasta/

lumn start [name:tag]             Registra com tag de versão (ex: cancelamentos:1.2)
                                  Sem tag, registra como :latest

lumn stop <id|name>               Para a execução do workflow (desativa triggers)
lumn delete <id|name>             Remove o workflow do daemon permanentemente
lumn restart <id|name>            Para e reinicia (aplica mudanças no arquivo)

── Observabilidade ─────────────────────────────────────────────────────────────

lumn list                         Tabela de todos os workflows no daemon:
                                  ID · NAME · VERSION · STATUS · LAST RUN · FAILS · NEXT RUN

lumn logs                         Stream de logs de todos os workflows em tempo real
                                  (comportamento pm2 logs — segue continuamente, Ctrl+C para sair)
lumn logs <id|name>               Stream do workflow específico
lumn logs --lines <n>             Exibe as últimas N linhas antes de seguir (padrão: 15)
lumn logs <id|name> --no-follow   Exibe logs históricos sem seguir (modo batch)
lumn logs <id|name> --since 1h    Filtra por janela de tempo
lumn logs <id|name> --level error Filtra por nível (debug|info|warn|error)
lumn logs <id|name> --step <nome> Logs de um step específico do flow

lumn watch                        TUI de todos os workflows ativos (Q para sair)
lumn watch <id|name>              TUI do workflow específico:
                                  · Painel superior: estado do DAG em ASCII (nodes por status)
                                  · Painel inferior: stream de logs em tempo real
                                  · Itens em processamento por step: [pipe:sap] 12/50

── Plugins ──────────────────────────────────────────────────────────────────────

lumn plugin add <ref>             Instala plugin
                                  Formatos: pretodev/outlook
                                            pretodev/plugins/outlook
                                            pretodev/outlook@v2.1.0
                                            gitlab.com/usuario/plugin
lumn plugin remove <nome>         Desinstala plugin
lumn plugin list                  Lista plugins instalados com versão e namespace
lumn plugin update                Atualiza todos respeitando o lumn.lock

── Credenciais ──────────────────────────────────────────────────────────────────

lumn credential add <key>         Setup guiado pelo plugin (OAuth, form, API key)
lumn credential list              Lista chaves (nunca valores)
lumn credential renew <key>       Renova credencial expirada
lumn credential remove <key>      Remove do vault
lumn credential export            Exporta vault criptografado de forma portável
lumn credential import <file>     Importa vault exportado

── Daemon ───────────────────────────────────────────────────────────────────────

lumn daemon start                 Inicia o daemon em background
lumn daemon stop                  Para o daemon graciosamente
lumn daemon status                Saúde do daemon e sumário de workflows ativos

── Interface e integração ───────────────────────────────────────────────────────

lumn ui                           Abre a Web UI no browser padrão
lumn mcp                          Inicia servidor MCP (stdio)
lumn mcp --transport sse          Inicia servidor MCP em modo SSE

── Deploy ───────────────────────────────────────────────────────────────────────

lumn deploy                       Empacota em imagem Docker com todos os workflows
lumn bundle                       Cria artifact .tar.gz para deploy manual
```

#### Exemplos de `lumn start`

```sh
# Registra lumn.lua do diretório atual, nome = nome da pasta, versão = latest
lumn start

# Registra com nome explícito
lumn start cancelamentos

# Registra com tag de versão
lumn start cancelamentos:1.2

# Registra arquivo específico com nome e tag
lumn start cancelamentos:1.2 -f cancelamentos.lua

# Registra pasta específica
lumn start pedidos -f order_cancel/

# Em um CI: registrar a pasta atual com a versão da build
lumn start meu-app:${BUILD_TAG} -f .
```

#### Exemplos de `lumn run`

```sh
# Executa lumn.lua do diretório atual (modo dev)
lumn run

# Executa uma instância registrada no daemon (fora do trigger)
lumn run cancelamentos
lumn run a3f92c1b  # por ID

# Executa arquivo ou pasta específica (modo dev)
lumn run -f order_cancel/
lumn run -f cancelamento.lua
```

### O daemon (`lumnd`)

O daemon é o processo que mantém a plataforma viva. Ele:

- Mantém os triggers ativos e os dispara no momento correto
- Executa workflows em resposta a triggers, com controle de concorrência
- Persiste o estado de execução no banco de dados local
- Serve a Web UI e os endpoints de webhook
- Expõe uma API HTTP local consumida pela CLI e pela Web UI

O daemon é projetado para ser leve. Em um ambiente com dezenas de workflows, consome menos de 50MB de RAM em idle. Não existe um cluster de workers — o paralelismo é gerenciado por goroutines dentro do processo único.

### `lumn list` — saída esperada

```
ID        NAME             VERSION    STATUS    LAST RUN          FAILS  NEXT RUN
a3f92c1b  cancelamentos    1.2        running   2025-03-29 08:15  0      —
b71d40e2  customer-sync    latest     idle      2025-03-29 07:00  2      09:00
c99a1f33  inventory-alert  latest     stopped   2025-03-28 23:00  0      —
```

Campos: `ID` (hash curto), `NAME`, `VERSION` (tag ou `latest`), `STATUS` (`running` · `idle` · `stopped` · `error`), `LAST RUN` (timestamp da última execução), `FAILS` (falhas acumuladas desde o último deploy), `NEXT RUN` (próxima execução agendada para schedulers).

### `lumn watch` — comportamento

`lumn watch` e `lumn watch <id|name>` abrem uma TUI (terminal user interface) em tempo real. A distinção em relação ao `lumn logs` é que o watch mostra o estado do DAG além dos logs — é o equivalente terminal da Web UI.

- Painel superior: estado do DAG em ASCII, nodes coloridos por status (aguardando · executando · concluído · erro)
- Painel inferior: stream de logs em tempo real do(s) workflow(s)
- Contadores por step visíveis durante execuções ativas: `[pipe:sap-customer] 12/50 items`
- `Q` sai do watch sem afetar nenhum workflow em execução

### Ciclo de desenvolvimento local

```sh
# 1. Criar projeto
mkdir order_cancel && cd order_cancel
# Criar lumn.lua manualmente ou copiar de um template existente

# 2. Adicionar plugins e credenciais
lumn plugin add lumn/outlook
lumn credential add outlook

# 3. Desenvolver (editar lumn.lua)
# Testar localmente — execução única, logs no terminal
lumn run

# Validar DAG antes de subir
lumn validate

# 4. Subir para o daemon
lumn daemon start
lumn start cancelamentos:1.0

# 5. Monitorar
lumn logs cancelamentos        # stream de logs, Ctrl+C para sair
lumn watch cancelamentos       # TUI com DAG + logs
lumn ui                        # Web UI no browser
```

---

## 13. Integração com IA via MCP

O lumn implementa o **Model Context Protocol (MCP)**, o padrão aberto para integração de ferramentas com LLMs. Qualquer assistente que suporte MCP — como Claude no Cursor, no Zed, ou em integrações customizadas — pode usar o lumn como ferramenta nativa.

### Como funciona

```sh
# Inicia o servidor MCP em modo stdio (para uso em editores como Cursor/Zed)
lumn mcp

# Inicia em modo SSE (para integrações via HTTP)
lumn mcp --transport sse --port 3001
```

O servidor MCP expõe ferramentas que o LLM pode chamar durante uma conversa:

| Ferramenta          | O que faz                                                       |
| ------------------- | --------------------------------------------------------------- |
| `list_workflows`    | Lista todos os workflows com status, última execução e triggers |
| `get_workflow`      | Retorna o conteúdo do `init.lua` de um workflow                 |
| `create_workflow`   | Gera um `init.lua` a partir de descrição em linguagem natural   |
| `run_workflow`      | Executa um workflow e retorna resultado estruturado por step    |
| `get_logs`          | Busca logs por workflow, step, status e intervalo de tempo      |
| `validate_workflow` | Valida sintaxe e DAG, retorna erros com sugestões de correção   |
| `install_plugin`    | Instala um plugin necessário para o workflow                    |
| `list_plugins`      | Lista plugins instalados com callables disponíveis e tipos      |
| `get_execution`     | Retorna o estado detalhado de uma execução específica           |

### Contexto injetado automaticamente

Quando o LLM chama `create_workflow`, o servidor MCP injeta automaticamente no contexto da geração:

- Lista de plugins instalados com todos os callables disponíveis
- Tipagem dos callables (parâmetros, tipos de retorno, exemplos)
- Workflows existentes no projeto (para manter consistência de estilo)
- Credenciais configuradas (apenas os nomes — para o LLM saber o que referenciar com `lumn.secret`)

Isso permite que o LLM gere `init.lua` correto e idiomático, usando os plugins já instalados, sem precisar adivinhar nomes ou assinar contratos de API.

### Caso de uso típico

Um desenvolvedor no Cursor descreve o que precisa em linguagem natural. O assistente, com o MCP server do lumn conectado, chama `list_plugins` para ver quais integrações estão disponíveis, depois `create_workflow` com a descrição e retorna um `init.lua` completo — pronto para ser validado com `lumn validate` e ativado com `lumn start`.

---

## 14. Deploy e operação

### Ambientes de operação

O lumn suporta três modelos de operação sem mudança no código dos workflows:

**Local (desenvolvimento)**
O daemon roda na máquina do desenvolvedor. Ideal para desenvolvimento, teste e automações pessoais que rodam continuamente em background.

**VPS / servidor dedicado**
O lumn roda como serviço systemd em um servidor Linux. Um script de setup automatizado instala o binário, configura o serviço e coloca o daemon para subir no boot.

```sh
curl -sSL https://lumn.dev/install.sh | sh
sudo systemctl enable lumnd && sudo systemctl start lumnd
```

**Container Docker**
Para ambientes com CI/CD ou Kubernetes, o lumn oferece imagem Docker oficial e o comando `lumn deploy` para criar imagens customizadas.

### `lumn deploy`

Empacota o projeto inteiro em uma imagem Docker:

```sh
lumn deploy --tag minha-loja:v1.2.0
```

A imagem gerada inclui:

- O binário do lumn
- Todos os `init.lua` e assets de cada pasta de workflow
- Os plugins instalados compilados para a arquitetura alvo
- A Web UI embutida

**Credenciais não são incluídas na imagem** — são injetadas via variáveis de ambiente ou montagem de volume no momento da execução:

```sh
docker run \
  -e LUMN_MASTER_PASSWORD=... \
  -v /secrets/vault.enc:/app/vault.enc \
  -p 8080:8080 \
  minha-loja:v1.2.0
```

### `lumn bundle`

Para ambientes sem Docker, cria um arquivo `.tar.gz` portável com o binário, os workflows e os plugins — pronto para deploy via rsync ou qualquer mecanismo de transfer.

### Multi-environment

O lumn suporta profiles de configuração para múltiplos ambientes:

```
meu-projeto/
├── lumn.config.lua           # configuração base
├── lumn.config.staging.lua   # overrides para staging
└── lumn.config.prod.lua      # overrides para produção
```

```sh
lumn start --env staging order_cancel/
```

### Observabilidade em produção

O daemon expõe métricas no formato Prometheus em `/metrics`:

- `lumn_executions_total` — contador por workflow e status
- `lumn_execution_duration_seconds` — histograma de duração por workflow
- `lumn_items_processed_total` — itens processados por step e workflow
- `lumn_trigger_fires_total` — disparos por trigger

Um endpoint `/health` retorna o status de saúde do daemon e de cada workflow registrado — pronto para uso como liveness/readiness probe no Kubernetes.

---

## 15. Arquitetura e design técnico

### Stack tecnológica

| Componente                | Tecnologia              | Justificativa                                                               |
| ------------------------- | ----------------------- | --------------------------------------------------------------------------- |
| Engine + CLI + Daemon     | Go                      | Binário único, goroutines para concorrência, cross-platform trivial         |
| Lua VM                    | gopher-lua              | Lua 5.1 em Go puro — sem CGo, compila em qualquer plataforma                |
| Storage de execução       | SQLite (modernc/sqlite) | Sem CGo, zero dependências externas, queries ad-hoc para logs e Data Tables |
| Vault de credenciais      | BBolt + AES-256-GCM     | K/V embarcado em Go, chave derivada via Argon2id                            |
| Comunicação plugin-engine | gRPC                    | Isolamento de processo, interface tipada, linguagem-agnóstico               |
| Web UI                    | SvelteKit + TypeScript  | Bundle pequeno, assets via `embed.FS` do Go — sem servidor Node separado    |
| Live updates              | WebSocket nativo        | Baixa latência, sem biblioteca extra                                        |
| MCP server                | mcp-go SDK              | Protocolo padrão de integração com LLMs                                     |

### Modelo de execução

O engine converte o `flow` de um `init.lua` em um **DAG (grafo acíclico dirigido)** de nodes. O executor percorre esse DAG em ordem topológica, respeitando dependências declaradas.

Cada node corresponde a um primitivo da DSL: `call`, `tap`, `pipe`, `set`, `filter`, `distinct`, `once`, `branch` e `parallel`. O `call` é sempre o node raiz — é o único primitivo que cria a lista de itens; todos os outros a consomem e transformam.

Quando a lista de itens fica vazia em qualquer ponto do fluxo — por um `filter` sem resultados, por uma fonte sem dados, ou por erros que ativaram `skip_item` em todos os itens — o workflow encerra com status `"empty"`. Nenhum step posterior é executado. Esse comportamento é automático e não requer primitivo especial.

O paralelismo é gerenciado por um worker pool de goroutines. Sub-pipelines dentro de `parallel {}` são submetidas ao pool e executadas concorrentemente; o executor aguarda todas antes de continuar para o próximo node.

O modelo é **batch**: o engine coleta todos os itens da fonte de dados antes de começar a processar. Isso simplifica o controle de concorrência e o tratamento de erros sem custo perceptível para os casos de uso alvo.

### O global `lumn` na VM Lua

Em vez de expor globals individuais por primitive, o lumn registra um único objeto global `lumn` na VM. Esse objeto é construído em Go e exposto à Lua via metatables. Ele combina:

- Primitivos da DSL (`exec`, `tap`, `set`, `filter`, `distinct`, `branch`, `once`, `parallel`)
- Funções utilitárias (`lumn.env`, `lumn.secret`, `lumn.bearer`, `lumn.date.*`)
- Namespaces de plugins carregados dinamicamente a partir do `lumn.lock`

Quando o engine carrega um `init.lua`, ele primeiro resolve todos os plugins declarados no projeto, registra seus callables sob `lumn.plugins.*`, e só então executa o arquivo. Isso garante que qualquer referência a `lumn.plugins.outlook` ou `lumn.ai.agent` esteja disponível no momento em que o arquivo é avaliado.

### Sandboxing da VM Lua

Workflows rodam em uma VM com globals restritos:

- **Bloqueados:** `io.*`, `os.exit`, `os.execute`, `load`, `loadfile`, `dofile`, `debug.*`
- **`require` controlado:** só carrega módulos Lua locais ao projeto (arquivos na pasta do workflow e em `_shared/`). Não pode carregar módulos do sistema.
- **Permitidos:** `math.*`, `string.*`, `table.*`, `os.time`, `os.clock`, `tostring`, `tonumber`, `ipairs`, `pairs`, `type`, `error`, `pcall`, `xpcall` e o global `lumn`

Plugins Go rodam em **processos separados** via gRPC. Um bug ou comportamento inesperado em um plugin não afeta o processo principal do engine nem outros plugins em execução.

### Estrutura do projeto

```
lumn/
├── cmd/
│   ├── lumn/          CLI binary
│   └── lumnd/         Daemon binary
│
├── internal/          Implementação privada — nunca importada por plugins
│   ├── dag/           DAG builder e validador
│   ├── engine/        Lifecycle de workflows (load, plan, run)
│   ├── executor/      Execução de steps e paralelismo (goroutine pool)
│   ├── lua/           VM Lua, sandbox e registro do global lumn
│   ├── trigger/       Sistema de triggers (scheduler, webhook, file watcher)
│   ├── store/         Camada de persistência (SQLite + BBolt)
│   ├── credentials/   Vault AES-256-GCM + interface de CredentialSpec
│   ├── plugin/        Loader, registry e host gRPC de plugins
│   └── server/        API HTTP + WebSocket do daemon
│
└── pkg/               API pública — importável por plugins externos
    ├── schema/        Sistema de tipos para items da pipeline
    ├── primitive/     NodeKinds e erros sentinela
    └── errkind/       Constantes de política de erro
```

---

## 16. Comparativo com alternativas

| Critério                         | lumn                   | n8n                  | Airflow            | Temporal           |
| -------------------------------- | ---------------------- | -------------------- | ------------------ | ------------------ |
| **Linguagem de definição**       | Lua DSL (arquivo)      | Visual (JSON)        | Python             | Go / Java / TS     |
| **Estrutura do projeto**         | Pastas com `init.lua`  | BD interno           | Módulos Python     | Código nativo      |
| **Versionamento com Git**        | Nativo                 | Export JSON          | Nativo             | Nativo             |
| **Testabilidade**                | `lumn run` local       | Manual               | pytest             | SDK próprio        |
| **Instalação**                   | Binário único          | Docker pesado        | pip + infra        | Cluster + servidor |
| **Curva de aprendizado**         | Baixa                  | Baixa                | Alta               | Muito alta         |
| **Lógica condicional**           | Lua nativo             | Expressões limitadas | Python nativo      | Código nativo      |
| **Plugin ecosystem**             | Git-based, aberto      | Integrado, fechado   | Providers          | SDK específico     |
| **Setup de credenciais**         | Guiado por plugin      | Manual / UI          | Connections UI     | Manual             |
| **Paralelismo**                  | `parallel {}` nativo   | Limitado             | TaskGroups         | Nativo             |
| **Encerramento por lista vazia** | Automático             | —                    | —                  | Manual             |
| **Deploy container**             | `lumn deploy`          | Docker Compose       | Helm chart         | Temporal Cloud     |
| **Caso de uso ideal**            | Integrações de negócio | Automações simples   | Pipelines de dados | Long-running       |

### Onde o lumn não é a escolha certa

Honestidade sobre limitações é parte do posicionamento:

- **Pipelines de dados em escala petabyte:** Airflow, Prefect e Dagster foram construídos para isso, com conectores nativos para Spark, BigQuery e similares.
- **Workflows de longa duração com state machine complexo e durabilidade:** Temporal tem primitivos específicos para isso (signals, queries, timers com garantia de execução após restart).
- **Usuários sem familiaridade com código:** n8n e Make são genuinamente melhores para quem não quer ver código.

---

## 17. Roadmap de produto

O desenvolvimento está organizado em fases, cada uma entregando valor utilizável de forma independente.

### Phase 0 — Foundation (semanas 1–6)

Engine core: VM Lua embarcada, global `lumn` com primitivos da DSL, DAG builder, executor sequencial e CLI básica (`init`, `run`, `validate`).

**Milestone:** `lumn run order_cancel/` executa o workflow de cancelamento do tutorial no terminal, mostrando o log de cada step, com output correto e erros com stack trace.

### Phase 1 — Runtime local (semanas 7–14)

Daemon (`lumnd`), triggers (scheduler, webhook, file watcher), paralelismo via goroutine pool, credential vault com `CredentialSpec` para plugins, e execution logs em SQLite.

**Milestone:** `lumn start order_cancel/` ativa o scheduler. O workflow roda a cada 15 minutos. `lumn logs order_cancel` mostra o histórico de execuções.

### Phase 2 — Visual environment (semanas 15–22)

Web UI com DAG visualizer em tempo real via WebSocket, execution history completo, step inspector com `res`/`item` por execução, credential manager UI e Data Tables.

**Milestone:** O desenvolvedor abre `lumn ui`, clica num step com erro e vê o stack trace, o valor de `res` e o `item` exatamente como estavam no momento da falha.

### Phase 3 — Plugin ecosystem (semanas 23–28)

Plugin registry baseado em Git com formato `usuario/plugin` e `usuario/path/plugin`, `lumn.lock` para reprodutibilidade, Plugin SDK completo com `CredentialSpec`, biblioteca padrão e sandboxing por subprocess gRPC.

**Milestone:** `lumn plugin add pretodev/outlook` instala o plugin. `lumn credential add outlook` abre o browser para OAuth. `lumn.plugins.outlook {}` funciona no `init.lua` sem configuração adicional.

### Phase 4 — MCP + AI (semanas 29–32)

Servidor MCP com todas as ferramentas documentadas, context injection automático de plugins e credenciais disponíveis, e suporte a criação de workflows via linguagem natural.

**Milestone:** Descrever o workflow de cancelamento ao Claude no Cursor. Receber um `init.lua` correto usando os plugins instalados, pronto para `lumn validate` e `lumn start`.

### Phase 5 — Deploy e produção (semanas 33–40)

`lumn deploy`, Docker multi-stage com imagem < 50MB, VPS setup script com systemd, métricas Prometheus, multi-environment com profiles e backup/restore do vault.

**Milestone:** `lumn deploy --tag minha-loja:v1.0.0 && docker run ... minha-loja:v1.0.0` — todos os workflows rodam, UI acessível, métricas em `/metrics`.

---

## 18. Posicionamento open source

### Licença

lumn é distribuído sob a licença **Apache 2.0** — uso comercial livre, incluindo rodar em infraestrutura de clientes e criar serviços gerenciados, com obrigação de preservar avisos de copyright.

### O que nunca será fechado

Por princípio do projeto, os seguintes componentes são e sempre serão open source:

- Engine core e DAG builder
- Runtime Lua e primitivos da DSL
- CLI (`lumn`) e daemon (`lumnd`)
- Sistema de triggers
- Credential vault e interface `CredentialSpec`
- Plugin SDK (Go e Lua)
- Biblioteca padrão de plugins

### Modelo de sustentabilidade

A sustentabilidade a longo prazo pode ser explorada por caminhos que não comprometem o core:

- **lumn Cloud:** hosting gerenciado do daemon com UI, backups automáticos, domínios customizados para webhooks e equipe de suporte dedicada
- **Enterprise plugins:** conectores para sistemas corporativos proprietários que não fazem sentido como open source
- **Suporte e consultoria:** para times que queiram adotar a plataforma com acompanhamento

### Contribuição e estabilidade de API

Novos primitivos da DSL passam por um processo de RFC (Request for Comments) público antes de serem aceitos. A estabilidade da API de plugins é uma garantia explícita: plugins escritos para a v1 da interface `Plugin` funcionam em todas as versões v1.x sem modificação.

O `CONTRIBUTING.md` cobre setup do ambiente de desenvolvimento, convenções de código, processo de review, como propor novos primitivos via RFC e como submeter um plugin para a biblioteca padrão.

---

_Este documento descreve a visão do produto lumn. Para a especificação técnica de implementação, consulte `SPEC.md`. Para o status atual de desenvolvimento, consulte `CHANGELOG.md`._
