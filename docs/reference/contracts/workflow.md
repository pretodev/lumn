# Workflow

> Estrutura da tabela retornada pelo arquivo de entrada de um workflow.

## Definicao

O arquivo de entrada (ex: `lumn.lua`, `init.lua`) deve retornar uma tabela com a seguinte estrutura:

```lua
return {
  triggers = { ... },  -- opcional
  flow     = { ... },  -- obrigatorio
}
```

## Campos

| Campo      | Tipo          | Obrigatorio | Descricao                                         |
|------------|---------------|-------------|-----------------------------------------------------|
| `flow`     | `FlowNode[]`  | Sim         | Array de nos do pipeline. Define o processamento.  |
| `triggers` | `Trigger[]`   | Nao         | Array de triggers. Se ausente, apenas execucao manual e permitida. |

## Regras do `flow`

1. `flow` deve ser uma tabela-array.
2. Se `flow` nao estiver vazio:
   - O **primeiro no** deve ser `call`.
   - Nenhum outro `call` pode aparecer apos o primeiro.
   - Cada no deve ser um primitivo valido: `call`, `set`, `filter` ou `tap`.
3. Um `flow` vazio e valido — o workflow executa sem processar itens.

## Regras de `triggers`

1. `triggers` e uma tabela-array de triggers criados via `lumn.triggers.*`.
2. Multiplos triggers podem coexistir no mesmo workflow.
3. Se `triggers` estiver ausente ou vazio, o comportamento equivale a `{ lumn.triggers.manual {} }`.
4. Validacao de triggers ocorre no `lumn start`, nao no `lumn validate`.

## Resolucao do arquivo de entrada

| Situacao                 | Arquivo                | Prioridade |
|--------------------------|------------------------|------------|
| `lumn run` (sem args)    | `./lumn.lua`           | Default    |
| `-f <pasta>/`            | `<pasta>/init.lua`     | 1a         |
| `-f <pasta>/` (sem init) | `<pasta>/lumn.lua`     | 2a         |
| `-f <arquivo.lua>`       | `<arquivo.lua>`        | Exato      |

## Exemplos

```lua
-- Workflow minimo
return {
  flow = {
    call { exec = lumn.from({ "a", "b", "c" }) },
  }
}
```

```lua
-- Workflow completo com triggers
local fetch = require("fetch_orders")
local notify = require("notifications")

return {
  triggers = {
    lumn.triggers.scheduler { interval = "15m" },
    lumn.triggers.webhook { path = "/hooks/orders" },
    lumn.triggers.manual {},
  },

  flow = {
    call {
      exec = fetch,
      on_data = function(order)
        return { id = order.id, total = order.amount }
      end
    },

    filter {
      condition = function(item)
        return item.total > 100
      end
    },

    set {
      to = function(item)
        item.processed = true
        return item
      end
    },

    tap { exec = notify },
  }
}
```

## Output da execucao

O resultado de `lumn run` e um JSON com a seguinte estrutura:

```json
{
  "workflow": "order_cancel",
  "version": "",
  "status": "ok",
  "items_in": 5,
  "items_out": 3,
  "errors": [],
  "duration_ms": 42
}
```

| Campo         | Tipo     | Descricao                                    |
|---------------|----------|-----------------------------------------------|
| `status`      | `string` | `"ok"`, `"empty"` ou `"error"`               |
| `items_in`    | `int`    | Itens produzidos pelo `call`                  |
| `items_out`   | `int`    | Itens restantes apos o pipeline              |
| `errors`      | `array`  | Lista de erros (se houver)                   |
| `duration_ms` | `int`    | Duracao da execucao em milissegundos         |

## Ver tambem

- [call](../primitives/call.md), [set](../primitives/set.md), [filter](../primitives/filter.md), [tap](../primitives/tap.md) — primitivos do pipeline
- [Triggers](../triggers/scheduler.md) — tipos de trigger disponiveis
- [Callable](callable.md) — contrato do callable usado em `call` e `tap`
