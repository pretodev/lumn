# filter

> Remove itens do batch que nao satisfazem uma condicao.

## Assinatura

```lua
filter { condition = fun(item: any): boolean }
```

## Propriedades

| Nome        | Tipo                        | Obrigatorio | Descricao                                      |
|-------------|-----------------------------|--------------|-------------------------------------------------|
| `condition` | `fun(item: any): boolean`   | Sim          | Funcao que retorna `true` para manter o item ou `false` para descarta-lo. |

## Comportamento

1. Para cada item do batch, invoca `condition(item)`.
2. Se retornar `true`, o item permanece no batch.
3. Se retornar `false`, o item e removido.
4. Se o batch ficar vazio apos a filtragem, os steps seguintes nao sao executados e o status final da execucao e `"empty"`.
5. Se o batch ja estiver vazio antes do filter, o step e ignorado.

## Exemplos

```lua
-- Manter apenas pedidos com valor alto
filter {
  condition = function(item)
    return item.total > 100
  end
}
```

```lua
-- Filtrar por status
filter {
  condition = function(item)
    return item.status == "pending" or item.status == "retry"
  end
}
```

```lua
-- Filtrar com base em dados do trigger
filter {
  condition = function(item)
    local trigger = lumn.trigger_data()
    if trigger.type == "manual" then
      return true -- manter todos em execucao manual
    end
    return item.priority >= 3
  end
}
```

## Erros comuns

| Codigo                  | Situacao                                                |
|-------------------------|---------------------------------------------------------|
| `ERR_INVALID_SIGNATURE` | `condition` nao esta presente ou nao e uma funcao.       |

## Ver tambem

- [set](set.md) — transforma itens sem remover
- [call](call.md) — produz os itens iniciais do pipeline
