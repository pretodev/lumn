---@meta

---Namespace global do runtime lumn.
---Contém funcoes utilitárias, acesso ao estado de execucao e construtores de triggers.
---@class lumn
---@field triggers lumn.triggers Construtores de triggers para workflows.
lumn = {}

---Cria um callable que retorna os itens fornecidos como fonte de dados.
---Util para testes e desenvolvimento local.
---
---O callable retornado tem `name = "lumn.from"` e uma funcao `run`
---que sempre retorna a tabela de itens fornecida.
---
---### Exemplo
---```lua
---local source = lumn.from({
---  { id = 1, name = "Pedido A" },
---  { id = 2, name = "Pedido B" },
---})
---
---return {
---  flow = {
---    call { exec = source },
---  }
---}
---```
---
---@param items any[] Tabela-array de itens a serem retornados pelo callable.
---@return Callable callable Callable com `name = "lumn.from"`.
function lumn.from(items) end

---Recupera um valor do estado de execucao.
---
---Disponivel apenas durante a execucao de um workflow (dentro de funcoes `run`,
---`to`, `condition` ou `on_data`). Fora desse contexto, gera erro de runtime.
---
---### Exemplo
---```lua
---set {
---  to = function(item)
---    local count = lumn.get("processed_count") or 0
---    lumn.set("processed_count", count + 1)
---    return item
---  end
---}
---```
---
---@param key string Chave do valor a recuperar.
---@return any value Valor armazenado, ou `nil` se a chave nao existir.
function lumn.get(key) end

---Armazena um valor no estado de execucao.
---
---Disponivel apenas durante a execucao de um workflow. O estado persiste entre
---os steps de uma mesma execucao, mas nao e compartilhado entre execucoes diferentes.
---Passar `nil` como valor remove a chave.
---
---### Exemplo
---```lua
---tap {
---  exec = {
---    name = "init_state",
---    run = function()
---      lumn.set("started_at", os.time())
---    end
---  }
---}
---```
---
---@param key string Chave para armazenar o valor.
---@param value any Valor a armazenar (ou `nil` para remover).
function lumn.set(key, value) end

---Retorna os dados de contexto do trigger que disparou a execucao atual.
---
---Cada tipo de trigger produz uma tabela com campos especificos.
---Fora de uma execucao via daemon (ex: `lumn run`), retorna `{ type = "none" }`.
---
---### Exemplo
---```lua
---call {
---  exec = {
---    name = "check_trigger",
---    run = function()
---      local trigger = lumn.trigger_data()
---      if trigger.type == "webhook" then
---        return trigger.body
---      end
---      return {}
---    end
---  }
---}
---```
---
---@return TriggerContext context Tabela com dados do trigger. O campo `type` indica a origem.
function lumn.trigger_data() end
