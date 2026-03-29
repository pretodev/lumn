# Spec for Daemon and Trigger System

branch: claude/feature/daemon-triggers

## Summary

Implementar o daemon (`lumnd`) como processo background responsavel por manter workflows ativos, gerenciar triggers e executar workflows sob demanda. A CLI (`lumn`) se comunica com o daemon via API HTTP local para registrar, listar, executar e parar workflows.

Esta fase introduz quatro tipos de trigger — scheduler, webhook, file watcher e execucao direta — e os comandos CLI necessarios para operar o daemon: `start`, `stop`, `status`, `exec`, `daemon start`, `daemon stop` e `daemon status`.

O daemon persiste workflows registrados e estado de execucao em SQLite, garantindo que workflows sobrevivam a restarts do processo. A arquitetura segue o modelo do Docker: o daemon e o runtime de execucao e a CLI e apenas um client que se comunica com ele.

## Decisions

- **Comunicacao CLI-Daemon via API HTTP local** — usamos `http://localhost:<porta>` em vez de socket Unix. Motivo: compatibilidade nativa com Windows (named pipes exigiriam abstracoes extras) e com todas as plataformas sem codigo condicional. HTTP tambem facilita debug (curl, browser) e e a mesma tecnologia que o daemon ja precisara para servir webhooks. A porta padrao sera configuravel (default: `6890`).
- **Persistencia em SQLite via modernc/sqlite** — sem CGo, conforme o Documento de Visao. Workflows registrados, estado de triggers e fila de execucao sao persistidos. Um restart do daemon restaura automaticamente todos os workflows ativos.
- **Webhook sem autenticacao nesta fase** — o servidor HTTP de webhooks roda em localhost. HMAC-SHA256 fica para fase futura quando secrets estiverem implementados.
- **`lumn exec` como comando de disparo manual** — separado de `lumn run` (que continua sendo execucao standalone sem daemon). `lumn exec` envia um request ao daemon para disparar o workflow via trigger de execucao direta.
- **Fila de execucao com enfileiramento** — se um workflow esta em execucao quando o proximo trigger dispara, a execucao e enfileirada (FIFO). Nao ha skip nem execucao paralela do mesmo workflow.
- **Reutilizacao do engine existente** — o daemon usa `internal/engine` e `internal/executor` como biblioteca para executar workflows. A logica de execucao nao e duplicada.
- **Sem Web UI nesta fase** — a interface e exclusivamente via CLI.

## Functional Requirements

### Daemon (`lumnd` / `lumn daemon`)

- `lumn daemon start` inicia o processo `lumnd` em background
  - O daemon escuta na porta HTTP configuravel (default `6890`)
  - Ao iniciar, restaura workflows ativos do SQLite e reativa seus triggers
  - Escreve um PID file para que a CLI saiba se o daemon esta rodando
  - Logs do daemon vao para arquivo em `~/.lumn/lumnd.log` (ou diretorio configuravel)
- `lumn daemon stop` envia sinal de shutdown gracioso ao daemon
  - Aguarda execucoes em andamento finalizarem (com timeout configuravel)
  - Desativa todos os triggers antes de encerrar
  - Remove o PID file
- `lumn daemon status` exibe informacoes de saude do daemon
  - Se esta rodando ou nao
  - Porta em uso
  - Numero de workflows ativos
  - Uptime

### API HTTP interna do daemon

O daemon expoe endpoints REST consumidos pela CLI:

| Endpoint | Metodo | Descricao |
|----------|--------|-----------|
| `/api/v1/health` | GET | Health check do daemon |
| `/api/v1/workflows` | GET | Lista todos os workflows registrados |
| `/api/v1/workflows` | POST | Registra um novo workflow (start) |
| `/api/v1/workflows/:id` | DELETE | Remove workflow do daemon (stop) |
| `/api/v1/workflows/:id/exec` | POST | Dispara execucao via trigger manual |
| `/api/v1/workflows/:id/status` | GET | Status detalhado de um workflow |
| `/hooks/*path` | ANY | Endpoints de webhook dos workflows |

### CLI — Novos comandos

- **`lumn start <pasta/>`** — Carrega o workflow, valida, registra no daemon e ativa triggers
  - Envia o caminho absoluto do workflow para o daemon via POST `/api/v1/workflows`
  - O daemon carrega o `init.lua`, valida e inicia os triggers
  - Retorna erro se o daemon nao estiver rodando
  - Retorna erro se o workflow ja estiver registrado
- **`lumn stop <workflow-id>`** — Desativa triggers e remove o workflow do daemon
  - Envia DELETE para `/api/v1/workflows/:id`
  - Aguarda execucao em andamento (se houver) antes de remover
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
```

- `path` e o diretorio a ser monitorado (caminho absoluto)
- `pattern` e opcional — filtro glob para nomes de arquivo
- `event` aceita: `"create"`, `"modify"`, `"delete"`, `"any"` (default: `"any"`)
- O daemon usa filesystem notifications (fsnotify ou equivalente) para monitorar
- Ao detectar evento correspondente, enfileira execucao do workflow
- Informacoes do evento (nome do arquivo, tipo de evento) sao passadas como contexto do trigger

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

### Fila de execucao

- Cada workflow tem sua propria fila FIFO
- Quando um trigger dispara e o workflow ja esta em execucao, a nova execucao vai para a fila
- Execucoes enfileiradas sao processadas em ordem apos a execucao corrente terminar
- A fila tem limite configuravel (default: 10). Execucoes alem do limite sao descartadas com log de warning
- O estado da fila e persistido em SQLite (sobrevive a restarts do daemon)

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

lumn status
  → CLI envia GET /api/v1/workflows
  → Daemon retorna lista com status de cada workflow e triggers
  → CLI formata em tabela no terminal
```

### Estrutura de diretorios do daemon

```
~/.lumn/
├── lumnd.db          SQLite database
├── lumnd.pid         PID file do processo daemon
├── lumnd.log         Log file do daemon
└── lumnd.conf        Configuracao opcional (porta, limites, etc.)
```

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
- Workflow atualizado no disco enquanto esta registrado no daemon (nao recarrega automaticamente — precisa `lumn stop` + `lumn start`)
- PID file existe mas processo nao esta rodando (stale PID)

## Acceptance Criteria

- `lumn daemon start` inicia o processo em background e persiste PID file
- `lumn daemon stop` encerra o daemon graciosamente
- `lumn daemon status` reporta saude quando daemon esta rodando e erro claro quando nao esta
- `lumn start <pasta/>` registra workflow e ativa triggers no daemon
- `lumn stop <workflow-id>` desativa triggers e remove workflow do daemon
- `lumn status` exibe tabela com todos os workflows, seus triggers e estado
- `lumn exec <workflow-id>` dispara workflow com trigger manual e retorna JSON report
- Um workflow com `lumn.triggers.scheduler { interval = "15m" }` executa automaticamente a cada 15 minutos
- Um workflow com `lumn.triggers.scheduler { cron = "..." }` executa no horario correto
- Um workflow com `lumn.triggers.webhook { path = "/hooks/test" }` executa ao receber POST em `http://localhost:6890/hooks/test`
- Um workflow com `lumn.triggers.file_watcher { path = "...", event = "create" }` executa ao criar arquivo no diretorio
- Workflows sem triggers aceitam execucao apenas via `lumn exec`
- Quando o daemon reinicia, todos os workflows ativos sao restaurados com seus triggers
- Execucoes sao enfileiradas quando o workflow ja esta em execucao
- Historico de execucoes e persistido em SQLite e consultavel via `lumn status`
- Erros de validacao de trigger geram mensagens claras na CLI
- O daemon funciona em Windows e Linux sem codigo condicional para comunicacao

## Open Questions

- Estrategia de debounce para file watcher: intervalo fixo (ex: 500ms) ou configuravel por trigger?
- Limite de retencao do historico de execucoes: rotacionar por quantidade, por data, ou ambos?
- Formato exato do `lumnd.conf`: Lua (consistente com o projeto), TOML, ou flags de linha de comando?
- `lumn restart <workflow-id>` deve ser implementado nesta fase ou basta `stop` + `start`?
- O contexto do trigger (body do webhook, info do file event) deve ser passado para o workflow via qual mecanismo? (candidato: `lumn.trigger_context()` ou campo especial no state)
- Comportamento quando `lumn start` e chamado e o daemon nao esta rodando: iniciar o daemon automaticamente ou apenas retornar erro?

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

Scenario: Erro ao usar CLI sem daemon rodando
  Given o daemon nao esta rodando
  When o desenvolvedor executa "lumn start pedidos/"
  Then o stderr mostra mensagem indicando que o daemon nao esta rodando
  And o exit code e diferente de 0

Scenario: Rejeitar trigger com configuracao invalida
  Given o daemon esta rodando
  And existe um workflow com trigger scheduler { interval = "15m", cron = "* * * * *" }
  When o desenvolvedor executa "lumn start" para esse workflow
  Then o stderr mostra erro indicando que interval e cron sao mutuamente exclusivos
  And o workflow nao e registrado
```
