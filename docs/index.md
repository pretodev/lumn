# lumn — Documento de Visão do Produto

> **Status:** Draft v0.2
> **Última atualização:** Março 2025
> **Audiência:** Colaboradores, early adopters, investidores técnicos

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

Cada workflow é uma **pasta** com um arquivo `init.lua` que retorna uma table de definição. Essa convenção mantém os workflows organizados, permite que arquivos auxiliares (templates, schemas, módulos locais) fiquem junto ao workflow que os usa, e torna o projeto navegável como qualquer outro projeto de software.

```
meu-projeto/
├── order_cancel/
│   ├── init.lua           ← definição do workflow
│   ├── templates/
│   │   ├── aprovado.html
│   │   └── negado.html
│   └── utils.lua          ← funções auxiliares locais
│
├── customer_sync/
│   └── init.lua
│
├── inventory_alert/
│   └── init.lua
│
└── lumn.lock              ← versões fixas dos plugins
```

O `init.lua` retorna uma table com a definição completa do workflow. Não existe convenção mágica de nome de função — o arquivo é um módulo Lua comum.

### O global `lumn`

As ferramentas, integrações e primitivos da plataforma são acessados através do global `lumn`, injetado pelo runtime no momento da execução. Não existe `require` para recursos da plataforma — `lumn` é o namespace único de tudo que o engine oferece.

```lua
-- Correto: acesso via global lumn
local agent = lumn.ai.agent { ... }
local client = lumn.http.client { ... }

-- Incorreto: require é para módulos Lua do projeto
local utils = require "order_cancel.utils"   -- isso funciona para código local
```

Essa distinção é deliberada: `require` carrega arquivos do disco (código do projeto), `lumn.*` acessa recursos do runtime (plugins, primitivos, credenciais).

### Modelo de pipeline

Um workflow opera sobre uma **lista de itens** que flui por uma sequência de primitivos. Cada primitivo recebe a lista, faz algo com ela, e passa o resultado para o próximo.

```
[emails] → exec → set → filter → distinct → exec → set → branch → [sent]
```

Esse modelo é intuitivo para qualquer desenvolvedor que já usou `Array.map/filter` em JavaScript ou pipes em Unix. Quando a lista de itens fica vazia em qualquer ponto do fluxo — por um `filter` sem resultados, por uma fonte sem dados, ou por erros que descartaram todos os itens — o workflow encerra naturalmente, sem erro. Não existe um primitivo especial para isso; é o comportamento padrão do runtime.

### Primitivos da DSL

A DSL oferece um vocabulário pequeno e ortogonal. Cada primitivo tem um contrato único e não se sobrepõe a nenhum outro:

| Primitivo  | O que faz                                                                       | Muta o item?            |
| ---------- | ------------------------------------------------------------------------------- | ----------------------- |
| `exec`     | Executa um callable (plugin); o resultado fica disponível no próximo `set`      | Não                     |
| `tap`      | Efeito colateral puro; resultado é descartado                                   | Nunca                   |
| `set`      | Lê `res` (output do `exec` anterior), `item` e `ctx`; retorna o item atualizado | Sim                     |
| `filter`   | Remove itens onde a condição retorna falso                                      | Não                     |
| `distinct` | Remove duplicatas por chave                                                     | Não                     |
| `branch`   | Roteia para sub-pipeline baseado em condição                                    | Condicional             |
| `once`     | Executa um callable uma única vez para todo o lote; salva no contexto           | Não                     |
| `parallel` | Executa sub-pipelines concorrentemente e converge                               | Depende do sub-pipeline |

### O primitivo `set`

`set` é o primitivo central de transformação. Ele substitui o que outras ferramentas chamam de `map`, `transform` e `merge` — funções diferentes com a mesma intenção. A assinatura unificada deixa explícito o que está acontecendo em qualquer situação:

```lua
set(function(res, item, ctx)
  -- res: output do callable executado pelo exec imediatamente anterior
  --      nil se não houve exec antes deste set
  -- item: estado atual do item na pipeline
  -- ctx: contexto global do workflow (compartilhado entre todos os items)

  item.campo = res.valor
  return item
end)
```

**Após um `exec`:** `res` contém o retorno do callable. É onde você extrai os dados da resposta de uma API e popula campos do item.

**Sem `exec` anterior:** `res` é `nil`. Você usa `set` apenas com `item` e `ctx` para calcular valores derivados, formatar strings, ou qualquer transformação puramente local.

**Com acesso ao contexto:** `ctx` está sempre disponível. Você pode ler o access token OAuth em `ctx.access_token`, ou qualquer outro valor salvo por `once`.

### Estado por item vs. estado global

Uma distinção central no modelo é a separação entre dois tipos de estado:

**`item`** é o objeto que carrega os dados de um elemento específico da pipeline — um e-mail, um pedido, um registro. Cada item tem sua própria cópia independente. Mutações em um item nunca afetam outros.

**`ctx`** (context) é o estado compartilhado entre todos os items de uma execução, e acessível diretamente como terceiro argumento de `set`. O access token OAuth é o exemplo canônico: obtido uma única vez via `once`, disponível para todos os items subsequentes.

### Exemplo comentado

O workflow abaixo processa cancelamentos de pedidos recebidos por e-mail. Ele serve como referência de como os primitivos se combinam em algo concreto:

```lua
-- order_cancel/init.lua

-- ── Declaração de componentes ───────────────────────────────────────────────
-- Instâncias de plugins configuradas uma vez e reutilizadas no flow.
-- Acessadas via global lumn — não via require.

local outlook = lumn.outlook.client {
  key = "outlook.cancelamentos",   -- referência ao credential store
}

local agent_extract = lumn.ai.agent {
  system_message = "Extraia os dados do formulário. Retorne somente JSON.",
  model = lumn.ai.model.azure_openai {
    name = "gpt-4o",
    key  = "azure.openai",
  },
  output_schema = {
    pedidoId = { type = "string",  required = true  },
    cpf      = { type = "cpf",     required = true  },
    email    = { type = "email",   required = false },
  },
  on_invalid = "skip",
}

local sap = lumn.http.client {
  base_url = lumn.env("SAP_BASE_URL"),
  headers  = { ["Ocp-Apim-Subscription-Key"] = lumn.env("OCP_KEY") },
}

local get_token = lumn.http.post {
  url  = "https://login.microsoftonline.com/token",
  body = {
    client_id     = lumn.env("SAP_CLIENT_ID"),
    client_secret = lumn.secret("SAP_CLIENT_SECRET"),
    grant_type    = "client_credentials",
  },
}

local send_mail = lumn.sendgrid.mail {
  sender_email = "no-reply@bemoldigital.com.br",
  sender_name  = "Bemol Digital",
}

-- ── Definição do workflow ────────────────────────────────────────────────────

return {
  id      = "order_cancel",
  version = "1.0.0",

  triggers = {
    lumn.triggers.scheduler { interval = "15m" },
  },

  context = {
    access_token = nil,
  },

  flow = {

    -- 1. Busca todos os e-mails não lidos
    exec(outlook.message.list { folder = "Inbox", unread = true }),

    -- 2. Extrai campos do e-mail para o item
    -- res = lista de e-mails retornada pelo exec acima
    set(function(res, item, ctx)
      item.email_id    = res.id
      item.received_at = res.received_datetime
      item.email_body  = res.body.content
      return item
    end),

    -- 3. Arquiva o e-mail na pasta Processando (efeito colateral, item não muda)
    tap(outlook.message.move {
      folder = "Processando",
      select = function(item) return item.email_id end,
    }),

    -- 4. Extrai dados estruturados do HTML via IA
    exec(agent_extract, {
      select = function(item) return item.email_body end,
    }),

    -- 5. Popula o item com os dados extraídos
    set(function(res, item, ctx)
      item.pedido_id  = res.pedidoId
      item.client_cpf = res.cpf
      item.client_email = res.email
      return item
    end),

    -- 6. Remove duplicatas do mesmo lote (mesmo pedido em dois e-mails)
    distinct(function(item) return item.pedido_id end),

    -- 7. Descarta itens sem CPF válido
    -- Se não sobrar nenhum item, o workflow encerra naturalmente aqui
    filter(function(item)
      return item.client_cpf ~= nil and #item.client_cpf == 11
    end),

    -- 8. Obtém token OAuth — uma única vez para todos os itens
    once(get_token, {
      into   = "access_token",
      select = function(res) return res.access_token end,
    }),

    -- 9. Consulta nível do cliente na API SAP
    exec(sap.get {
      path  = "/Customer/CustomerList",
      auth  = lumn.bearer("access_token"),
      query = function(item)
        return { ["$filter"] = "CPF eq '" .. item.client_cpf .. "'" }
      end,
    }),

    -- 10. Popula nível do cliente
    set(function(res, item, ctx)
      item.client_id    = res.customerId
      item.client_level = res.customerLevel
      return item
    end),

    -- 11. Calcula se o pedido está dentro do prazo de cancelamento
    set(function(res, item, ctx)
      local days = (item.client_level == "diamond") and 14 or 7
      item.is_within_period = lumn.date.now() <= lumn.date.add(item.order_date, days)
      item.deadline_days    = days
      return item
    end),

    -- 12. Envia e-mail de acordo com o prazo
    branch {
      condition = function(item) return item.is_within_period end,

      on_true = tap(send_mail {
        to       = function(item) return item.client_email end,
        template = "order_cancel/templates/aprovado.html",
        data     = function(item) return { nome = item.client_name, pedido = item.pedido_id } end,
      }),

      on_false = tap(send_mail {
        to       = function(item) return item.client_email end,
        template = "order_cancel/templates/negado.html",
        data     = function(item) return { nome = item.client_name, prazo = item.deadline_days } end,
      }),
    },

  },

  on_error = {
    default       = "skip_item",
    agent_extract = "manual_review",
    get_token     = "fail",
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

O lumn vem com uma biblioteca padrão de plugins mantida pela equipe principal:

| Plugin              | Descrição                                         | Credential command                 |
| ------------------- | ------------------------------------------------- | ---------------------------------- |
| `lumn/http`         | Cliente HTTP genérico com auth, retry e paginação | —                                  |
| `lumn/smtp`         | Envio de e-mail via SMTP                          | `lumn credential add smtp`         |
| `lumn/sendgrid`     | Envio via SendGrid API                            | `lumn credential add sendgrid`     |
| `lumn/outlook`      | E-mails via Microsoft Graph API                   | `lumn credential add outlook`      |
| `lumn/gdrive`       | Arquivos via Google Drive API                     | `lumn credential add gdrive`       |
| `lumn/slack`        | Mensagens para canais Slack                       | `lumn credential add slack`        |
| `lumn/aws-s3`       | Objetos no Amazon S3                              | `lumn credential add aws`          |
| `lumn/openai`       | Modelos GPT via OpenAI API                        | `lumn credential add openai`       |
| `lumn/azure-openai` | GPT via Azure OpenAI Service                      | `lumn credential add azure-openai` |
| `lumn/ai`           | Primitivos de IA: agent, schema, model            | —                                  |
| `lumn/data`         | Acesso às Data Tables integradas                  | —                                  |

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

```
lumn init <nome>               Cria uma pasta de workflow com init.lua scaffold
lumn run  <pasta/>             Executa uma vez, sem daemon (modo dev, logs no terminal)
lumn validate <pasta/>         Valida sintaxe Lua e DAG sem executar

lumn start <pasta/>            Registra workflow no daemon e ativa triggers
lumn stop  <workflow-id>       Desativa triggers e remove do daemon
lumn restart <workflow-id>     Recarrega o workflow (aplica mudanças no init.lua)
lumn status                    Lista todos os workflows ativos com status e próximo run
lumn logs  <workflow-id>       Exibe logs da última execução em tempo real

lumn plugin add    <ref>       Instala plugin (pretodev/outlook, pretodev/plugins/gdrive)
lumn plugin remove <nome>      Desinstala plugin
lumn plugin list               Lista plugins instalados com versão
lumn plugin update             Atualiza todos os plugins respeitando o lumn.lock

lumn credential add    <key>   Executa o fluxo de setup guiado do plugin para <key>
lumn credential list           Lista credenciais (apenas nomes, nunca valores)
lumn credential renew  <key>   Renova uma credencial expirada
lumn credential remove <key>   Remove uma credencial do vault
lumn credential export         Exporta o vault criptografado de forma portável
lumn credential import <file>  Importa um vault exportado

lumn daemon start              Inicia o daemon em background
lumn daemon stop               Para o daemon graciosamente
lumn daemon status             Exibe saúde do daemon e workflows ativos

lumn ui                        Abre a Web UI no browser padrão
lumn mcp                       Inicia o servidor MCP (stdio por padrão)
lumn mcp --transport sse       Inicia o MCP em modo SSE (para integrações HTTP)

lumn deploy                    Empacota tudo em imagem Docker versionada
lumn bundle                    Cria artifact portável (.tar.gz) para deploy manual
```

### O daemon (`lumnd`)

O daemon é o processo que mantém a plataforma viva. Ele:

- Mantém os triggers ativos e os dispara no momento correto
- Executa workflows em resposta a triggers, com controle de concorrência
- Persiste o estado de execução no banco de dados local
- Serve a Web UI e os endpoints de webhook
- Expõe uma API HTTP local consumida pela CLI e pela Web UI

O daemon é projetado para ser leve. Em um ambiente com dezenas de workflows, consome menos de 50MB de RAM em idle. Não existe um cluster de workers — o paralelismo é gerenciado por goroutines dentro do processo único.

### Ciclo de desenvolvimento local

```sh
# Criar um novo workflow
lumn init order_cancel
cd order_cancel

# Adicionar os plugins necessários
lumn plugin add lumn/outlook
lumn plugin add lumn/openai

# Configurar credenciais (fluxo guiado pelo plugin)
lumn credential add outlook
lumn credential add openai

# Desenvolver o init.lua...
# Testar sem daemon (execução única, logs no terminal)
lumn run order_cancel/

# Validar DAG e sintaxe
lumn validate order_cancel/

# Subir daemon e ativar o workflow
lumn daemon start
lumn start order_cancel/

# Monitorar
lumn ui
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

Quando a lista de itens fica vazia em qualquer ponto do fluxo — por um `filter` sem resultados, por uma fonte sem dados, ou por erros que ativaram `skip_item` em todos os itens — o workflow encerra com status `"empty"`. Nenhum step posterior é executado. Esse comportamento é automático e não requer primitivo especial.

O paralelismo é gerenciado por um worker pool de goroutines. Sub-pipelines dentro de `parallel {}` são submetidas ao pool e executadas concorrentemente; o executor aguarda todas antes de continuar para o próximo node.

O modelo é **batch**: o engine coleta todos os itens da fonte de dados antes de começar a processar. Isso simplifica o controle de concorrência e o tratamento de erros sem custo perceptível para os casos de uso alvo.

### O global `lumn` na VM Lua

Em vez de expor globals individuais por primitive, o lumn registra um único objeto global `lumn` na VM. Esse objeto é construído em Go e exposto à Lua via metatables. Ele combina:

- Primitivos da DSL (`exec`, `tap`, `set`, `filter`, `distinct`, `branch`, `once`, `parallel`)
- Funções utilitárias (`lumn.env`, `lumn.secret`, `lumn.bearer`, `lumn.date.*`)
- Namespaces de plugins carregados dinamicamente a partir do `lumn.lock`

Quando o engine carrega um `init.lua`, ele primeiro resolve todos os plugins declarados no projeto, registra seus callables sob `lumn.<plugin>.*`, e só então executa o arquivo. Isso garante que qualquer referência a `lumn.outlook.client` ou `lumn.ai.agent` esteja disponível no momento em que o arquivo é avaliado.

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

**Milestone:** `lumn plugin add pretodev/outlook` instala o plugin. `lumn credential add outlook` abre o browser para OAuth. `lumn.outlook.client {}` funciona no `init.lua` sem configuração adicional.

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
