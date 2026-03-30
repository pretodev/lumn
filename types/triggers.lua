---@meta

-- Construtores de triggers disponíveis em `lumn.triggers.*`.
-- Triggers definem quando e como um workflow e disparado pelo daemon.

---@class lumn.triggers
lumn.triggers = {}

---Opcoes do trigger scheduler baseado em intervalo.
---@class SchedulerIntervalOpts
---@field interval string Intervalo entre execucoes. Aceita sufixos: `"s"` (segundos), `"m"` (minutos), `"h"` (horas). Ex: `"15m"`, `"1h"`, `"30s"`.

---Opcoes do trigger scheduler baseado em cron.
---@class SchedulerCronOpts
---@field cron string Expressao cron de 5 campos. Ex: `"0 9 * * MON-FRI"`.
---@field timezone? string Timezone IANA para a expressao cron. Ex: `"America/Sao_Paulo"`, `"UTC"`.

---Cria um trigger de agendamento. O workflow e executado periodicamente com base
---em um intervalo simples ou uma expressao cron.
---
---`interval` e `cron` sao mutuamente exclusivos. Fornecer ambos gera erro de validacao.
---
---### Exemplos
---```lua
----- A cada 15 minutos
---lumn.triggers.scheduler { interval = "15m" }
---
----- Dias uteis as 9h (horario de Sao Paulo)
---lumn.triggers.scheduler {
---  cron     = "0 9 * * MON-FRI",
---  timezone = "America/Sao_Paulo",
---}
---```
---
---@param opts SchedulerIntervalOpts|SchedulerCronOpts
---@return Trigger
function lumn.triggers.scheduler(opts) end

---Opcoes do trigger webhook.
---@class WebhookOpts
---@field path string Caminho do endpoint HTTP. Ex: `"/hooks/novo-pedido"`.
---@field method? string Metodo HTTP aceito. Default: `"POST"`.

---Cria um trigger de webhook. O workflow e executado quando o daemon recebe
---uma requisicao HTTP no caminho registrado (servidor em `localhost:6890`).
---
---O corpo da requisicao fica disponivel via `lumn.trigger_data()`.
---
---### Exemplo
---```lua
---lumn.triggers.webhook {
---  path   = "/hooks/novo-pedido",
---  method = "POST",
---}
---```
---
---@param opts WebhookOpts
---@return Trigger
function lumn.triggers.webhook(opts) end

---Opcoes do trigger file_watcher.
---@class FileWatcherOpts
---@field path string Caminho absoluto do diretorio a monitorar.
---@field pattern? string Filtro glob para nomes de arquivo. Ex: `"*.csv"`.
---@field event? "create"|"modify"|"delete"|"any" Tipo de evento a observar. Default: `"any"`.
---@field debounce? string Intervalo de debounce para agrupar eventos rapidos. Aceita `"ms"` e `"s"`. Default: `"500ms"`.

---Cria um trigger de monitoramento de arquivos. O workflow e executado quando
---um evento de filesystem correspondente e detectado no diretorio especificado.
---
---Detalhes do evento ficam disponiveis via `lumn.trigger_data()`.
---
---### Exemplos
---```lua
----- Monitorar criacao de CSVs
---lumn.triggers.file_watcher {
---  path    = "/data/importacoes",
---  pattern = "*.csv",
---  event   = "create",
---}
---
----- Com debounce customizado
---lumn.triggers.file_watcher {
---  path     = "/data/importacoes",
---  pattern  = "*.csv",
---  event    = "create",
---  debounce = "2s",
---}
---```
---
---@param opts FileWatcherOpts
---@return Trigger
function lumn.triggers.file_watcher(opts) end

---Cria um trigger manual. O workflow so e executado sob demanda via `lumn exec`.
---
---Se o campo `triggers` do workflow estiver ausente ou vazio, um trigger manual
---e assumido implicitamente.
---
---### Exemplo
---```lua
---lumn.triggers.manual {}
---```
---
---@param opts? table Sem opcoes necessarias. Aceita tabela vazia.
---@return Trigger
function lumn.triggers.manual(opts) end
