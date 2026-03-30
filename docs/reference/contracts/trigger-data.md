# Trigger Data

> Formato dos dados retornados por `lumn.trigger_data()` para cada tipo de trigger.

## Visao geral

`lumn.trigger_data()` retorna uma tabela cujo campo `type` identifica o trigger que disparou a execucao. Cada tipo tem campos adicionais especificos.

## Tipos

### scheduler

```lua
{
  type         = "scheduler",
  scheduled_at = "2025-03-15T09:00:00Z",  -- horario agendado (ISO 8601)
  fired_at     = "2025-03-15T09:00:01Z",  -- horario real de disparo (ISO 8601)
}
```

| Campo          | Tipo     | Descricao                                |
|----------------|----------|-------------------------------------------|
| `type`         | `string` | Sempre `"scheduler"`.                     |
| `scheduled_at` | `string` | Horario em que a execucao estava agendada. Formato ISO 8601. |
| `fired_at`     | `string` | Horario em que o trigger efetivamente disparou. Formato ISO 8601. |

### webhook

```lua
{
  type    = "webhook",
  body    = { items = { ... } },
  headers = { ["Content-Type"] = "application/json" },
  method  = "POST",
  path    = "/hooks/novo-pedido",
}
```

| Campo     | Tipo     | Descricao                                          |
|-----------|----------|----------------------------------------------------|
| `type`    | `string` | Sempre `"webhook"`.                                |
| `body`    | `table`  | Corpo da requisicao HTTP, parseado como tabela Lua. |
| `headers` | `table`  | Headers da requisicao HTTP.                        |
| `method`  | `string` | Metodo HTTP (ex: `"POST"`, `"GET"`).               |
| `path`    | `string` | Caminho do endpoint registrado.                    |

### file_watcher

```lua
{
  type  = "file_watcher",
  file  = "dados.csv",
  event = "create",
  path  = "/data/importacoes",
}
```

| Campo   | Tipo     | Descricao                                                  |
|---------|----------|-------------------------------------------------------------|
| `type`  | `string` | Sempre `"file_watcher"`.                                   |
| `file`  | `string` | Nome do arquivo que disparou o evento.                     |
| `event` | `string` | Tipo de evento: `"create"`, `"modify"` ou `"delete"`.      |
| `path`  | `string` | Caminho do diretorio monitorado.                           |

### manual

```lua
{
  type = "manual",
}
```

| Campo  | Tipo     | Descricao              |
|--------|----------|------------------------|
| `type` | `string` | Sempre `"manual"`.     |

### none

Retornado quando a execucao nao foi disparada por nenhum trigger (ex: `lumn run` standalone).

```lua
{
  type = "none",
}
```

| Campo  | Tipo     | Descricao            |
|--------|----------|----------------------|
| `type` | `string` | Sempre `"none"`.     |

## Notas

- A tabela retornada e uma **copia** — modificacoes nao afetam o estado interno.
- `lumn.trigger_data()` pode ser chamada multiplas vezes; cada chamada retorna uma nova copia.
- Independente do mecanismo de `lumn.get` / `lumn.set`.

## Ver tambem

- [lumn.trigger_data](../namespace/lumn.trigger_data.md) — funcao que retorna estes dados
- [scheduler](../triggers/scheduler.md), [webhook](../triggers/webhook.md), [file_watcher](../triggers/file_watcher.md), [manual](../triggers/manual.md) — declaracao de cada trigger
