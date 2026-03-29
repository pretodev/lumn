# Spec for Daemon and Trigger System

branch: claude/feature/daemon-triggers

## Summary

Implementar o daemon (`lumnd`) como processo background responsavel por manter workflows ativos, gerenciar triggers e executar workflows sob demanda. A CLI (`lumn`) se comunica com o daemon via API HTTP sobre transporte nativo da plataforma (Unix socket em Linux/macOS, named pipe em Windows) para registrar, listar, executar e parar workflows.

Esta fase introduz quatro tipos de trigger — scheduler, webhook, file watcher e execucao direta — e os comandos CLI necessarios para operar o daemon: `start`, `stop`, `restart`, `status`, `exec`, `daemon start`, `daemon stop` e `daemon status`.

O daemon persiste workflows registrados e estado de execucao em SQLite, garantindo que workflows sobrevivam a restarts do processo. A arquitetura segue o modelo do Docker: o daemon e o runtime de execucao e a CLI e apenas um client que se comunica com ele. A comunicacao CLI-Daemon replica o padrao do Docker — protocolo HTTP sobre transporte nativo do SO.

## Decisions

- **Comunicacao CLI-Daemon via HTTP sobre transporte nativo do SO (modelo Docker)** — o protocolo e HTTP, mas o transporte e nativo da plataforma: Unix socket (`~/.lumn/lumnd.sock`) em Linux/macOS e named pipe (`\\.\pipe\lumnd`) em Windows. Esse e exatamente o modelo do Docker: `docker.sock` em Linux/macOS, `//./pipe/docker_engine` em Windows, com HTTP como protocolo sobre ambos. Go suporta isso nativamente via custom `net.Dialer` no `http.Client` — sem bibliotecas extras. Vantagens sobre TCP puro: sem conflito de porta, sem exposicao acidental na rede, e permissoes de acesso controladas pelo filesystem. O servidor HTTP de webhooks continua em TCP (`localhost:6890`) por ser externo.
- **Persistencia em SQLite via modernc/sqlite** — sem CGo, conforme o Documento de Visao. Workflows registrados, estado de triggers e fila de execucao sao persistidos. Um restart do daemon restaura automaticamente todos os workflows ativos.
- **Webhook sem autenticacao nesta fase** — o servidor HTTP de webhooks roda em localhost. HMAC-SHA256 fica para fase futura quando secrets estiverem implementados.
- **`lumn exec` como comando de disparo manual** — separado de `lumn run` (que continua sendo execucao standalone sem daemon). `lumn exec` envia um request ao daemon para disparar o workflow via trigger de execucao direta.
- **Fila de execucao com enfileiramento** — se um workflow esta em execucao quando o proximo trigger dispara, a execucao e enfileirada (FIFO). Nao ha skip nem execucao paralela do mesmo workflow.
- **Reutilizacao do engine existente** — o daemon usa `internal/engine` e `internal/executor` como biblioteca para executar workflows. A logica de execucao nao e duplicada.
- **Sem Web UI nesta fase** — a interface e exclusivamente via CLI.
- **`lumnd.conf` em Lua** — o arquivo de configuracao do daemon usa Lua, consistente com todo o ecossistema do projeto (workflows sao Lua, config do workspace e Lua). Justificativa: (1) o runtime Lua ja existe no binario — nao adiciona dependencia de parser; (2) permite validacao com as mesmas regras do sandbox; (3) desenvolvedores do lumn ja conhecem a sintaxe; (4) tables Lua sao mais expressivas que TOML/INI para configuracoes aninhadas sem a verbosidade de JSON/YAML. O arquivo retorna uma table, mesmo padrao de `init.lua`.
- **File watcher com debounce configuravel e default sensato** — debounce padrao de 500ms, configuravel por trigger via campo `debounce` na table do file_watcher. Valor suficiente para agrupar rajadas tipicas de filesystem (editores que fazem write+rename, ou copias de multiplos arquivos).
- **Retencao de historico com rotacao dual** — rotacao por quantidade (default: 1000 execucoes por workflow) e por tempo (default: 30 dias). O que for atingido primeiro dispara a limpeza. Ambos os limites sao configuraveis em `lumnd.conf`. O default de 30 dias garante visibilidade historica do daemon por periodo operacionalmente relevante.
- **`lumn restart` incluido nesta fase** — implementado como sequencia atomica de stop + start. Se o stop falhar, o restart aborta e reporta o erro. Se o stop concluir mas o start falhar, o workflow fica parado e o erro e reportado.
- **Contexto do trigger via `lumn.trigger_data()`** — funcao dedicada no global `lumn` que retorna uma table especifica ao tipo de trigger que disparou a execucao. Cada tipo de trigger produz um objeto proprio: webhook retorna `{ body, headers, method, path }`, file_watcher retorna `{ file, event, path }`, scheduler retorna `{ scheduled_at, fired_at }`, manual retorna `{}`. A funcao e read-only e nao depende de get/set.
- **Erro explicito quando daemon nao esta rodando (modelo Docker)** — quando a CLI tenta se comunicar com o daemon e a conexao falha (socket/pipe nao existe ou recusa conexao), retorna erro claro: `Cannot connect to the lumn daemon at <path>. Is the daemon running? (lumn daemon start)`. Mesmo padrao do `docker: Cannot connect to the Docker daemon`.

## Functional Requirements

### Daemon (`lumnd` / `lumn daemon`)

- `lumn daemon start` inicia o processo `lumnd` em background
  - O daemon escuta em transporte nativo do SO:
    - Linux/macOS: Unix socket em `~/.lumn/lumnd.sock`
    - Windows: named pipe em `\\.\pipe\lumnd`
  - Adicionalmente, inicia servidor HTTP TCP em `localhost:6890` (configuravel) para webhooks
  - Ao iniciar, restaura workflows ativos do SQLite e reativa seus triggers
  - Escreve um PID file para que a CLI saiba se o daemon esta rodando
  - Logs do daemon vao para arquivo em `~/.lumn/lumnd.log` (ou diretorio configuravel)
- `lumn daemon stop` envia sinal de shutdown gracioso ao daemon
  - Aguarda execucoes em andamento finalizarem (com timeout configuravel)
  - Desativa todos os triggers antes de encerrar
  - Remove o PID file e o socket/pipe
- `lumn daemon status` exibe informacoes de saude do daemon
  - Se esta rodando ou nao
  - Transporte em uso (socket path ou pipe name)
  - Porta HTTP de webhooks
  - Numero de workflows ativos
  - Uptime

### API HTTP interna do daemon

O daemon expoe endpoints REST sobre o transporte nativo (socket/pipe), consumidos pela CLI:

| Endpoint | Metodo | Descricao |
|----------|--------|-----------|
| `/api/v1/health` | GET | Health check do daemon |
| `/api/v1/workflows` | GET | Lista todos os workflows registrados |
| `/api/v1/workflows` | POST | Registra um novo workflow (start) |
| `/api/v1/workflows/:id` | DELETE | Remove workflow do daemon (stop) |
| `/api/v1/workflows/:id/restart` | POST | Restart do workflow (stop + start atomico) |
| `/api/v1/workflows/:id/exec` | POST | Dispara execucao via trigger manual |
| `/api/v1/workflows/:id/status` | GET | Status detalhado de um workflow |

Adicionalmente, o servidor HTTP TCP (`localhost:6890`) expoe:

| Endpoint | Metodo | Descricao |
|----------|--------|-----------|
| `/hooks/*path` | ANY | Endpoints de webhook dos workflows |

### CLI — Novos comandos

- **`lumn start <pasta/>`** — Carrega o workflow, valida, registra no daemon e ativa triggers
  - Envia o caminho absoluto do workflow para o daemon via POST `/api/v1/workflows`
  - O daemon carrega o `init.lua`, valida e inicia os triggers
  - Se o daemon nao estiver rodando, retorna: `Cannot connect to the lumn daemon at <socket/pipe>. Is the daemon running? (lumn daemon start)`
  - Retorna erro se o workflow ja estiver registrado
- **`lumn stop <workflow-id>`** — Desativa triggers e remove o workflow do daemon
  - Envia DELETE para `/api/v1/workflows/:id`
  - Aguarda execucao em andamento (se houver) antes de remover
- **`lumn restart <workflow-id>`** — Recarrega o workflow (aplica mudancas no init.lua)
  - Envia POST para `/api/v1/workflows/:id/restart`
  - Executa stop + start como sequencia atomica
  - Se o stop falhar, aborta e reporta erro
  - Se o stop concluir mas o start falhar, workflow fica parado e erro e reportado
- **`lumn status`** — Lista workflows com estado, tipo de trigger e proxima execucao
  - Formato tabular no terminal
  - Colunas: ID, Version, Status (active/inactive), Trigger, Next Run, Last Run, Last Status
  - Workflows ativos e inativos sao exibidos (inativos = registrados mas com trigger desativado, ou historico)
- **`lumn exec <workflow-id>`** — Dispara execucao imediata de um workflow registrado
  - So funciona para workflows com trigger do tipo `manual` (execucao direta)
  - Envia POST para `/api/v1/workflows/:id/exec`
  - Retorna o resultado da execucao (JSON report) no stdout, mesmo formato de `lumn run`

### Definicao de triggers no workflow

Triggers sao declarados no `init.lua` dentro do campo `triggers` da table retornada:

```lua
return {
  id      = "order_cancel",
  version = "1.0.0",

  triggers = {
    lumn.triggers.scheduler { interval = "15m" },
  },

  flow = { ... }
}
```

O campo `triggers` e uma table-array. Multiplos triggers podem coexistir no mesmo workflow. Se `triggers` estiver ausente ou vazio, o workflow so aceita execucao via `lumn exec` (trigger manual implicito).

### Trigger: Scheduler

```lua
-- Por intervalo simples
lumn.triggers.scheduler { interval = "15m" }
lumn.triggers.scheduler { interval = "1h" }
lumn.triggers.scheduler { interval = "30s" }

-- Por cron expression
lumn.triggers.scheduler { cron = "0 9 * * MON-FRI" }

-- Por cron com timezone
lumn.triggers.scheduler {
  cron     = "0 9 * * MON-FRI",
  timezone = "America/Manaus",
}
```

- `interval` aceita sufixos: `s` (segundos), `m` (minutos), `h` (horas)
- `cron` segue o formato cron padrao de 5 campos
- `interval` e `cron` sao mutuamente exclusivos — erro de validacao se ambos presentes
- O scheduler agenda a proxima execucao e enfileira no daemon
- O daemon persiste o proximo horario agendado no SQLite

### Trigger: Webhook

```lua
lumn.triggers.webhook {
  path   = "/hooks/novo-pedido",
  method = "POST",
}
```

- O daemon registra o path no servidor HTTP de webhooks
- Ao receber um request no path registrado, o daemon enfileira execucao do workflow
- O body do request HTTP e passado como input para o workflow (disponivel no `call.exec` como contexto do trigger)
- O response HTTP retorna `202 Accepted` com um ID de execucao
- `method` e opcional; default e `POST`
- Conflito de path entre workflows e erro de validacao

### Trigger: File Watcher

```lua
lumn.triggers.file_watcher {
  path    = "/data/importacoes",
  pattern = "*.csv",
  event   = "create",
}

-- Com debounce customizado
lumn.triggers.file_watcher {
  path     = "/data/importacoes",
  pattern  = "*.csv",
  event    = "create",
  debounce = "2s",
}
```

- `path` e o diretorio a ser monitorado (caminho absoluto)
- `pattern` e opcional — filtro glob para nomes de arquivo
- `event` aceita: `"create"`, `"modify"`, `"delete"`, `"any"` (default: `"any"`)
- `debounce` e opcional — intervalo para agrupar rajadas de eventos (default: `"500ms"`). Aceita sufixos `ms` e `s`
- O daemon usa filesystem notifications (fsnotify ou equivalente) para monitorar
- Ao detectar evento correspondente, aguarda janela de debounce e enfileira execucao do workflow
- Informacoes do evento (nome do arquivo, tipo de evento) sao passadas como contexto do trigger via `lumn.trigger_data()`

### Trigger: Execucao Direta (Manual)

```lua
lumn.triggers.manual {}
```

- Ou implicitamente, quando `triggers` esta ausente/vazio
- O workflow fica registrado no daemon, carregado e pronto para execucao
- So e disparado via `lumn exec <workflow-id>`
- Util para workflows que funcionam como "comandos" invocaveis sob demanda

### Registro de triggers no runtime Lua

- `lumn.triggers` e um namespace injetado no global `lumn`
- Cada funcao de trigger (`scheduler`, `webhook`, `file_watcher`, `manual`) retorna uma table com metadados do trigger
- A validacao dos triggers ocorre no momento do `lumn start`, nao no `lumn validate` (pois `validate` e standalone)
- Triggers invalidos (ex: `interval` e `cron` ao mesmo tempo, path vazio no file_watcher) geram erro de validacao com mensagem clara

### Contexto do trigger via `lumn.trigger_data()`

O workflow acessa informacoes do trigger que disparou a execucao via `lumn.trigger_data()`. A funcao retorna uma table read-only especifica ao tipo de trigger:

```lua
-- Dentro de qualquer callback do workflow:
local trigger = lumn.trigger_data()
```

Retornos por tipo de trigger:

| Trigger | Campos retornados |
|---------|-------------------|
| `scheduler` | `{ type = "scheduler", scheduled_at = "ISO8601", fired_at = "ISO8601" }` |
| `webhook` | `{ type = "webhook", body = table, headers = table, method = "POST", path = "/hooks/..." }` |
| `file_watcher` | `{ type = "file_watcher", file = "dados.csv", event = "create", path = "/data/importacoes" }` |
| `manual` | `{ type = "manual" }` |

- A funcao e read-only — modificacoes na table retornada nao afetam o estado interno
- Chamar `lumn.trigger_data()` fora de uma execucao via daemon (ex: `lumn run`) retorna `{ type = "none" }`
- Nao depende de `lumn.get`/`lumn.set` — e um mecanismo independente

### Fila de execucao

- Cada workflow tem sua propria fila FIFO
- Quando um trigger dispara e o workflow ja esta em execucao, a nova execucao vai para a fila
- Execucoes enfileiradas sao processadas em ordem apos a execucao corrente terminar
- A fila tem limite configuravel (default: 10). Execucoes alem do limite sao descartadas com log de warning
- O estado da fila e persistido em SQLite (sobrevive a restarts do daemon)

### Configuracao do daemon (`lumnd.conf`)

O arquivo de configuracao usa Lua, consistente com todo o ecossistema (workflows, config do workspace). Fica em `~/.lumn/lumnd.conf` e retorna uma table:

```lua
return {
  -- Porta do servidor HTTP para webhooks (default: 6890)
  webhook_port = 6890,

  -- Limite da fila de execucao por workflow (default: 10)
  queue_limit = 10,

  -- Timeout de shutdown em segundos (default: 30)
  shutdown_timeout = 30,

  -- Retencao de historico de execucoes
  retention = {
    max_executions = 1000,  -- por workflow (default: 1000)
    max_days       = 30,    -- dias (default: 30)
  },

  -- Nivel de log: "debug", "info", "warn", "error" (default: "info")
  log_level = "info",
}
```

- Se o arquivo nao existir, o daemon usa defaults
- Campos ausentes usam default individual
- Campos desconhecidos geram warning no log (nao erro, para forward-compatibility)
- A validacao do conf usa o mesmo sandbox Lua do projeto

### Retencao de historico de execucoes

- Rotacao dual: por quantidade (default: 1000 por workflow) e por tempo (default: 30 dias)
- O criterio atingido primeiro dispara a limpeza
- A limpeza roda periodicamente no daemon (a cada hora) e no startup
- Ambos os limites sao configuraveis em `lumnd.conf`
- O default de 30 dias garante visibilidade historica operacionalmente relevante

### Persistencia (SQLite)

O banco SQLite fica em `~/.lumn/lumnd.db` (ou diretorio configuravel). Tabelas:

- **`workflows`** — workflows registrados: id, version, path, status (active/stopped), created_at, updated_at
- **`triggers`** — triggers de cada workflow: workflow_id, type, config (JSON), next_run_at, status
- **`executions`** — historico de execucoes: id, workflow_id, trigger_type, status (queued/running/ok/error/empty), started_at, finished_at, report (JSON)
- **`queue`** — fila de execucoes pendentes: id, workflow_id, trigger_type, trigger_context (JSON), queued_at, priority

### Ciclo de vida de um workflow no daemon

```
lumn start <pasta/>
  → CLI envia POST /api/v1/workflows com path absoluto
  → Daemon carrega init.lua, valida, extrai triggers
  → Insere registro em workflows e triggers no SQLite
  → Ativa cada trigger:
      scheduler → calcula proximo run, agenda goroutine
      webhook   → registra rota no servidor HTTP
      file_watcher → inicia watcher no diretorio
      manual    → nenhuma acao (aguarda lumn exec)
  → Retorna confirmacao com workflow-id e triggers ativos

lumn exec <workflow-id>
  → CLI envia POST /api/v1/workflows/:id/exec
  → Daemon verifica se workflow existe e esta ativo
  → Enfileira execucao na fila do workflow
  → Worker pega da fila, executa via engine.RunTarget()
  → Persiste resultado em executions
  → Retorna report JSON para a CLI

lumn stop <workflow-id>
  → CLI envia DELETE /api/v1/workflows/:id
  → Daemon desativa triggers (para scheduler, remove rota webhook, para watcher)
  → Aguarda execucao em andamento (com timeout)
  → Marca workflow como stopped no SQLite
  → Retorna confirmacao

lumn restart <workflow-id>
  → CLI envia POST /api/v1/workflows/:id/restart
  → Daemon executa stop (desativa triggers, aguarda execucao)
  → Se stop falhou → aborta, retorna erro
  → Daemon recarrega init.lua do disco, valida, executa start
  → Se start falhou → workflow fica parado, retorna erro
  → Retorna confirmacao com triggers atualizados

lumn status
  → CLI envia GET /api/v1/workflows
  → Daemon retorna lista com status de cada workflow e triggers
  → CLI formata em tabela no terminal
```

### Estrutura de diretorios do daemon

```
~/.lumn/
├── lumnd.db          SQLite database
├── lumnd.sock        Unix socket (Linux/macOS)
├── lumnd.pid         PID file do processo daemon
├── lumnd.log         Log file do daemon
└── lumnd.conf        Configuracao Lua (porta, limites, retencao, etc.)
```

No Windows, o named pipe `\\.\pipe\lumnd` substitui o socket file (nao aparece no filesystem).

### Estrutura de codigo proposta

```
cmd/lumnd/
  main.go             Bootstrap do daemon: HTTP server, restaura estado, signal handling

internal/
  daemon/
    daemon.go         Lifecycle do daemon: start, stop, restore
    api.go            Handlers HTTP (REST endpoints)
    queue.go          Fila de execucao por workflow (FIFO)
    worker.go         Worker pool que consome da fila e executa via engine

  trigger/
    trigger.go        Interface Trigger e registry
    scheduler.go      Trigger de agendamento (interval + cron)
    webhook.go        Trigger de webhook HTTP
    filewatcher.go    Trigger de monitoramento de diretorio
    manual.go         Trigger de execucao direta (noop trigger)

  store/
    store.go          Camada de acesso ao SQLite
    migrations.go     Schema migrations
    models.go         Structs de dominio para workflows, triggers, executions

  cli/
    cli.go            Adicionar comandos: daemon, start, stop, status, exec
```

## Possible Edge Cases

- Daemon nao esta rodando quando CLI tenta `start`, `stop`, `exec` ou `status`
- Workflow ja registrado quando `lumn start` e chamado novamente
- Workflow com `init.lua` invalido (erro de sintaxe, estrutura errada)
- Trigger com configuracao invalida (interval + cron juntos, path inexistente no file_watcher)
- Dois workflows tentam registrar webhook no mesmo path
- Diretorio monitorado pelo file_watcher e deletado enquanto daemon esta rodando
- Fila de execucao atinge o limite maximo
- Daemon recebe shutdown enquanto existem execucoes em andamento
- Daemon reinicia e precisa restaurar schedulers com base no proximo horario persistido
- `lumn exec` chamado em workflow que nao tem trigger manual e nao tem triggers vazios
- Porta do daemon ja esta em uso por outro processo
- SQLite database corrompido ou inacessivel
- File watcher recebe rajada de eventos (debounce necessario)
- Workflow atualizado no disco enquanto esta registrado no daemon (nao recarrega automaticamente — precisa `lumn restart`)
- PID file existe mas processo nao esta rodando (stale PID)
- `lumnd.conf` com sintaxe Lua invalida — daemon deve iniciar com defaults e logar warning
- Restart falha no stop — workflow deve permanecer no estado anterior, nao ficar em estado inconsistente
- Restart falha no start (init.lua com erro) — workflow fica parado com erro claro

## Acceptance Criteria

- `lumn daemon start` inicia o processo em background e persiste PID file
- `lumn daemon stop` encerra o daemon graciosamente
- `lumn daemon status` reporta saude quando daemon esta rodando e erro claro quando nao esta
- `lumn start <pasta/>` registra workflow e ativa triggers no daemon
- `lumn stop <workflow-id>` desativa triggers e remove workflow do daemon
- `lumn restart <workflow-id>` recarrega workflow do disco e reativa triggers
- `lumn status` exibe tabela com todos os workflows, seus triggers e estado
- `lumn exec <workflow-id>` dispara workflow com trigger manual e retorna JSON report
- Um workflow com `lumn.triggers.scheduler { interval = "15m" }` executa automaticamente a cada 15 minutos
- Um workflow com `lumn.triggers.scheduler { cron = "..." }` executa no horario correto
- Um workflow com `lumn.triggers.webhook { path = "/hooks/test" }` executa ao receber POST em `http://localhost:6890/hooks/test`
- Um workflow com `lumn.triggers.file_watcher { path = "...", event = "create" }` executa ao criar arquivo no diretorio
- File watcher agrupa rajadas de eventos com debounce de 500ms (default), configuravel por trigger
- Workflows sem triggers aceitam execucao apenas via `lumn exec`
- Quando o daemon reinicia, todos os workflows ativos sao restaurados com seus triggers
- Execucoes sao enfileiradas quando o workflow ja esta em execucao
- Historico de execucoes e persistido em SQLite e consultavel via `lumn status`
- Rotacao de historico funciona por quantidade (1000) e por tempo (30 dias), o que for atingido primeiro
- `lumn.trigger_data()` retorna table especifica ao tipo de trigger que disparou a execucao
- `lumnd.conf` em Lua e carregado no startup; campos ausentes usam defaults
- Erros de validacao de trigger geram mensagens claras na CLI
- CLI sem daemon retorna erro claro no modelo Docker: `Cannot connect to the lumn daemon...`
- Comunicacao CLI-Daemon funciona em Windows (named pipe), Linux e macOS (Unix socket) sem divergencia de protocolo

## Open Questions

Todas as questoes levantadas na versao anterior foram resolvidas. Nao ha open questions pendentes nesta fase.

## Testing Guidelines

```gherkin
Scenario: Iniciar e parar o daemon
  Given o daemon nao esta rodando
  When o desenvolvedor executa "lumn daemon start"
  Then o processo lumnd inicia em background
  And um PID file e criado em ~/.lumn/lumnd.pid
  And "lumn daemon status" reporta o daemon como rodando
  When o desenvolvedor executa "lumn daemon stop"
  Then o processo encerra graciosamente
  And o PID file e removido

Scenario: Registrar workflow com trigger scheduler
  Given o daemon esta rodando
  And existe um workflow "pedidos" com trigger scheduler { interval = "30s" }
  When o desenvolvedor executa "lumn start pedidos/"
  Then "lumn status" mostra "pedidos" como active com trigger "scheduler"
  And o campo "Next Run" mostra o proximo horario agendado
  And apos 30 segundos o workflow e executado automaticamente

Scenario: Disparar workflow via webhook
  Given o daemon esta rodando
  And existe um workflow "novo-pedido" com trigger webhook { path = "/hooks/pedido", method = "POST" }
  And o workflow esta registrado com "lumn start"
  When uma requisicao POST e enviada para "http://localhost:6890/hooks/pedido"
  Then o daemon retorna 202 Accepted
  And o workflow e executado
  And o resultado aparece no historico via "lumn status"

Scenario: Monitorar pasta e disparar workflow
  Given o daemon esta rodando
  And existe um workflow "importacao" com trigger file_watcher { path = "/tmp/imports", event = "create" }
  And o workflow esta registrado com "lumn start"
  When um arquivo "dados.csv" e criado em "/tmp/imports"
  Then o workflow e executado automaticamente
  And o historico mostra o trigger como "file_watcher"

Scenario: Executar workflow sob demanda via lumn exec
  Given o daemon esta rodando
  And existe um workflow "hello-world" sem triggers (manual implicito)
  And o workflow esta registrado com "lumn start"
  When o desenvolvedor executa "lumn exec hello-world"
  Then o stdout contem JSON report com status "ok"
  And o historico de execucoes e atualizado

Scenario: Enfileirar execucao quando workflow ja esta rodando
  Given o daemon esta rodando
  And o workflow "pesado" esta em execucao
  When um novo trigger dispara para "pesado"
  Then a execucao e adicionada a fila
  And apos a execucao corrente terminar, a proxima inicia automaticamente

Scenario: Daemon restaura workflows apos restart
  Given o daemon esta rodando com workflow "pedidos" ativo (trigger scheduler)
  When o daemon e parado e reiniciado
  Then "lumn status" mostra "pedidos" como active
  And o scheduler e reativado com o proximo horario correto

Scenario: Restart recarrega workflow do disco
  Given o daemon esta rodando com workflow "pedidos" ativo
  And o desenvolvedor altera o init.lua de "pedidos" (ex: muda interval de 15m para 30m)
  When o desenvolvedor executa "lumn restart pedidos"
  Then o workflow e parado e reiniciado com a nova configuracao
  And "lumn status" mostra o novo intervalo de 30m

Scenario: Acessar contexto do trigger via lumn.trigger_data()
  Given o daemon esta rodando
  And um workflow usa lumn.trigger_data() dentro de um set.to callback
  And o workflow tem trigger webhook { path = "/hooks/data" }
  When uma requisicao POST com body JSON e enviada para o webhook
  Then lumn.trigger_data() retorna table com type="webhook", body, headers, method e path
  And o workflow consegue usar os dados do body no processamento

Scenario: Erro ao usar CLI sem daemon rodando
  Given o daemon nao esta rodando
  When o desenvolvedor executa "lumn start pedidos/"
  Then o stderr mostra "Cannot connect to the lumn daemon" com instrucao para iniciar
  And o exit code e diferente de 0

Scenario: Rejeitar trigger com configuracao invalida
  Given o daemon esta rodando
  And existe um workflow com trigger scheduler { interval = "15m", cron = "* * * * *" }
  When o desenvolvedor executa "lumn start" para esse workflow
  Then o stderr mostra erro indicando que interval e cron sao mutuamente exclusivos
  And o workflow nao e registrado
```
