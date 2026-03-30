# lumn.get

> Recupera um valor do estado de execucao.

## Assinatura

```lua
lumn.get(key: string) -> any
```

## Parametros

| Nome  | Tipo     | Descricao                     |
|-------|----------|-------------------------------|
| `key` | `string` | Chave do valor a recuperar.   |

## Retorno

O valor armazenado para a chave, ou `nil` se nao existir.

## Comportamento

1. Acessa o estado de execucao compartilhado entre todos os steps de uma mesma execucao.
2. Disponivel **apenas durante a execucao** — dentro de funcoes `run`, `to`, `condition` ou `on_data`.
3. Chamadas fora do contexto de execucao geram `ERR_RUNTIME`.
4. O estado **nao persiste** entre execucoes diferentes do mesmo workflow.

## Exemplos

```lua
-- Contar itens processados
set {
  to = function(item)
    local count = lumn.get("count") or 0
    lumn.set("count", count + 1)
    return item
  end
}
```

```lua
-- Acessar configuracao definida em um tap anterior
filter {
  condition = function(item)
    local threshold = lumn.get("min_total") or 0
    return item.total > threshold
  end
}
```

## Erros comuns

| Codigo        | Situacao                                                    |
|---------------|-------------------------------------------------------------|
| `ERR_RUNTIME` | Chamada fora do contexto de execucao (ex: no escopo global). |

## Ver tambem

- [lumn.set](lumn.set.md) — armazena valores no estado de execucao
