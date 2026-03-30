---@meta

-- Tipos compartilhados da DSL lumn.
-- Este arquivo define os contratos fundamentais usados por primitivos, triggers e workflows.

---Um callable e uma tabela que representa uma unidade de trabalho executavel.
---Todo callable deve ter um `name` nao-vazio e uma funcao `run`.
---
---### Exemplo
---```lua
---local fetch_orders = {
---  name = "fetch_orders",
---  description = "Busca pedidos pendentes da API",
---  run = function(input, state)
---    return { { id = 1, total = 100 }, { id = 2, total = 250 } }
---  end
---}
---```
---@class Callable
---@field name string Nome unico do callable (obrigatorio, nao pode ser vazio)
---@field description? string Descricao do callable (opcional)
---@field run fun(input: any, state: table): any Funcao executada pelo runtime (obrigatoria)

---Um no do pipeline de execucao. Retornado pelos primitivos `call`, `set`, `filter` e `tap`.
---@class FlowNode

---Definicao de um trigger. Retornado pelas funcoes em `lumn.triggers.*`.
---@class Trigger

---Estrutura de um workflow retornada pelo arquivo de entrada.
---
---### Exemplo
---```lua
---return {
---  triggers = {
---    lumn.triggers.scheduler { interval = "15m" },
---  },
---  flow = {
---    call { exec = lumn.from({ "a", "b" }) },
---    set { to = function(item) return string.upper(item) end },
---  }
---}
---```
---@class Workflow
---@field flow FlowNode[] Array de nos do pipeline (obrigatorio). Se nao-vazio, o primeiro no deve ser `call`.
---@field triggers? Trigger[] Array de triggers (opcional). Se ausente, apenas execucao manual e permitida.

---Contexto retornado por `lumn.trigger_data()`.
---O campo `type` indica qual trigger disparou a execucao.
---@class TriggerContext
---@field type "scheduler"|"webhook"|"file_watcher"|"manual"|"none" Tipo do trigger que disparou a execucao.

---Contexto de trigger do tipo scheduler.
---@class SchedulerTriggerContext : TriggerContext
---@field type "scheduler"
---@field scheduled_at string Horario agendado no formato ISO 8601.
---@field fired_at string Horario real de disparo no formato ISO 8601.

---Contexto de trigger do tipo webhook.
---@class WebhookTriggerContext : TriggerContext
---@field type "webhook"
---@field body table Corpo da requisicao HTTP.
---@field headers table Headers da requisicao HTTP.
---@field method string Metodo HTTP (ex: "POST").
---@field path string Caminho registrado do webhook (ex: "/hooks/novo-pedido").

---Contexto de trigger do tipo file_watcher.
---@class FileWatcherTriggerContext : TriggerContext
---@field type "file_watcher"
---@field file string Nome do arquivo que disparou o evento.
---@field event "create"|"modify"|"delete" Tipo de evento no filesystem.
---@field path string Caminho do diretorio monitorado.

---Contexto de trigger do tipo manual.
---@class ManualTriggerContext : TriggerContext
---@field type "manual"

---Contexto retornado quando a execucao nao foi disparada por nenhum trigger (ex: `lumn run`).
---@class NoneTriggerContext : TriggerContext
---@field type "none"
