# lumn.trigger_data

> Retorna os dados de contexto do trigger que disparou a execucao.

## Assinatura

```lua
lumn.trigger_data() -> TriggerContext
```

## Parametros

Nenhum.

## Retorno

Uma tabela com dados especificos do trigger. O campo `type` identifica a origem:

| `type`         | Campos adicionais                                              |
|----------------|----------------------------------------------------------------|
| `"scheduler"`  | `scheduled_at` (ISO 8601), `fired_at` (ISO 8601)              |
| `"webhook"`    | `body` (table), `headers` (table), `method` (string), `path` (string) |
| `"file_watcher"` | `file` (string), `event` (string), `path` (string)         |
| `"manual"`     | Nenhum                                                         |
| `"none"`       | Nenhum — retornado fora do daemon (ex: `lumn run`)             |

## Comportamento

1. Retorna uma **copia** dos dados do trigger — modificacoes na tabela retornada nao afetam o estado interno.
2. Pode ser chamada em qualquer ponto da execucao (dentro de `run`, `to`, `condition`, `on_data`).
3. Fora de uma execucao via daemon (ex: `lumn run` standalone), retorna `{ type = "none" }`.
4. Independente de `lumn.get` / `lumn.set` — mecanismo separado.

## Exemplos

```lua
-- Usar o body do webhook como fonte de dados
call {
  exec = {
    name = "webhook_source",
    run = function()
      local trigger = lumn.trigger_data()
      if trigger.type == "webhook" then
        return trigger.body.items or {}
      end
      return {}
    end
  }
}
```

```lua
-- Filtrar com base no tipo de trigger
filter {
  condition = function(item)
    local trigger = lumn.trigger_data()
    if trigger.type == "manual" then
      return true
    end
    return item.priority >= 3
  end
}
```

```lua
-- Registrar informacao do arquivo que disparou a execucao
tap {
  exec = {
    name = "log_trigger",
    run = function()
      local trigger = lumn.trigger_data()
      if trigger.type == "file_watcher" then
        print("Arquivo:", trigger.file, "Evento:", trigger.event)
      end
    end
  }
}
```

## Ver tambem

- [Trigger Data](../contracts/trigger-data.md) — definicao completa de cada tipo de contexto
- [Triggers](../triggers/scheduler.md) — como declarar triggers no workflow
