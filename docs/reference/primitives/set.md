# set

> Transforma cada item do batch aplicando uma funcao.

## Assinatura

```lua
set { to = fun(item: any): any }
```

## Propriedades

| Nome | Tipo                | Obrigatorio | Descricao                                   |
|------|---------------------|-------------|----------------------------------------------|
| `to` | `fun(item: any): any` | Sim       | Funcao que recebe o item atual e retorna o item transformado. |

## Comportamento

1. Para cada item do batch, invoca `to(item)`.
2. O valor retornado substitui o item original no batch.
3. Retornar `nil` gera erro de runtime — todo item deve ser transformado em um valor valido.
4. Se o batch estiver vazio, o step e ignorado.

## Exemplos

```lua
-- Adicionar campo a cada item
set {
  to = function(item)
    item.processed = true
    item.processed_at = os.time()
    return item
  end
}
```

```lua
-- Transformar o formato do item
set {
  to = function(item)
    return {
      label = string.format("%s (#%d)", item.name, item.id),
      value = item.total,
    }
  end
}
```

```lua
-- Usar estado de execucao
set {
  to = function(item)
    local seq = (lumn.get("seq") or 0) + 1
    lumn.set("seq", seq)
    item.sequence = seq
    return item
  end
}
```

## Erros comuns

| Codigo                  | Situacao                                         |
|-------------------------|--------------------------------------------------|
| `ERR_INVALID_SIGNATURE` | `to` nao esta presente ou nao e uma funcao.       |
| `ERR_RUNTIME`           | A funcao `to` retornou `nil` para um item.        |

## Ver tambem

- [filter](filter.md) — remove itens do batch com base em condicao
- [call](call.md) — produz os itens iniciais do pipeline
- [lumn.get / lumn.set](../namespace/lumn.get.md) — estado compartilhado entre steps
