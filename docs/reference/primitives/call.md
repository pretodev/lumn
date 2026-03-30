# call

> Invoca um callable e injeta os resultados como itens do pipeline.

## Assinatura

```lua
call { exec = Callable, on_data? = fun(result: any): any }
```

## Propriedades

| Nome      | Tipo                    | Obrigatorio | Descricao                                         |
|-----------|-------------------------|-------------|----------------------------------------------------|
| `exec`    | `Callable`              | Sim         | Callable a ser executado para produzir itens.      |
| `on_data` | `fun(result: any): any` | Nao         | Funcao que transforma cada resultado antes de entrar no batch. |

## Comportamento

1. Invoca `exec.run(nil, state)` — o callable recebe `nil` como input e o estado de execucao.
2. Se o resultado for uma tabela-array (indices inteiros consecutivos a partir de 1), cada elemento se torna um item do batch.
3. Se o resultado for um valor unico (nao-array), ele se torna o unico item do batch.
4. Se `on_data` estiver definida, cada resultado bruto passa pela funcao antes de entrar no batch.
5. Sem `on_data`, os resultados brutos se tornam itens diretamente.

## Regras de posicao

- `call` **deve ser o primeiro no** do `flow` quando o `flow` nao e vazio.
- Nenhum outro `call` pode aparecer apos o primeiro.

## Exemplos

```lua
-- Fonte estatica simples
call { exec = lumn.from({ "a", "b", "c" }) }
```

```lua
-- Com transformacao via on_data
call {
  exec = fetch_orders,
  on_data = function(order)
    return {
      id    = order.id,
      total = order.amount * 100,
    }
  end
}
```

```lua
-- Callable inline
call {
  exec = {
    name = "generate_items",
    run = function()
      return { 1, 2, 3, 4, 5 }
    end
  }
}
```

## Erros comuns

| Codigo                    | Situacao                                              |
|---------------------------|-------------------------------------------------------|
| `ERR_INVALID_SIGNATURE`   | `exec` nao e um callable valido (falta `name` ou `run`). |
| `ERR_INVALID_SIGNATURE`   | `on_data` esta presente mas nao e uma funcao.          |
| `ERR_CALLABLE_NOT_FOUND`  | O simbolo referenciado em `exec` nao foi encontrado.   |
| `ERR_STRUCTURE`           | `call` aparece em posicao diferente da primeira no `flow`. |

## Ver tambem

- [Callable](../contracts/callable.md) — contrato que `exec` deve satisfazer
- [tap](tap.md) — executa um callable como efeito colateral, sem alterar o batch
- [lumn.from](../namespace/lumn.from.md) — callable builtin para fonte de dados de teste
