# Callable

> Contrato que define uma unidade de trabalho executavel.

## Definicao

Um callable e uma tabela Lua com a seguinte estrutura:

```lua
{
  name        = string,                       -- obrigatorio, nao-vazio
  description = string,                       -- opcional
  run         = fun(input: any, state: table): any  -- obrigatorio
}
```

## Campos

| Campo         | Tipo                              | Obrigatorio | Descricao                                         |
|---------------|-----------------------------------|-------------|-----------------------------------------------------|
| `name`        | `string`                          | Sim         | Nome unico do callable. Deve ser nao-vazio. Usado em mensagens de erro e logs. |
| `description` | `string`                          | Nao         | Descricao legivel do que o callable faz.            |
| `run`         | `fun(input: any, state: table): any` | Sim      | Funcao executada pelo runtime.                      |

## Parametros de `run`

| Parametro | Tipo    | Descricao                                                            |
|-----------|---------|-----------------------------------------------------------------------|
| `input`   | `any`   | Item atual (no contexto de `call`: `nil`; no contexto de `tap`: copia do item). |
| `state`   | `table` | Tabela de estado de execucao (mesmo acessivel via `lumn.get`/`lumn.set`). |

## Retorno de `run`

- No contexto de `call`: o valor retornado se torna o batch de itens. Arrays geram multiplos itens; valores unicos geram um item.
- No contexto de `tap`: o retorno e **descartado**.

## Exemplos

```lua
-- Callable como variavel local
local fetch_orders = {
  name = "fetch_orders",
  description = "Busca pedidos pendentes",
  run = function(input, state)
    return {
      { id = 1, total = 150 },
      { id = 2, total = 80 },
    }
  end
}

return {
  flow = {
    call { exec = fetch_orders },
  }
}
```

```lua
-- Callable inline no primitivo
tap {
  exec = {
    name = "logger",
    run = function(item)
      print("Processado:", item.id)
    end
  }
}
```

```lua
-- Callable de modulo compartilhado
-- _shared/notifications.lua
return {
  name = "notify_slack",
  description = "Envia mensagem para o Slack",
  run = function(item)
    -- logica de notificacao
  end
}
```

## Validacao

O runtime valida callables durante a construcao do DAG:

| Verificacao                   | Erro                      |
|-------------------------------|---------------------------|
| Tabela sem campo `name`       | `ERR_INVALID_SIGNATURE`   |
| `name` vazio (`""`)           | `ERR_INVALID_SIGNATURE`   |
| Tabela sem campo `run`        | `ERR_INVALID_SIGNATURE`   |
| `run` nao e uma funcao        | `ERR_INVALID_SIGNATURE`   |
| Simbolo nao resolvido         | `ERR_CALLABLE_NOT_FOUND`  |

## Ver tambem

- [call](../primitives/call.md) — usa callable no campo `exec`
- [tap](../primitives/tap.md) — usa callable no campo `exec`
- [lumn.from](../namespace/lumn.from.md) — callable builtin para testes
