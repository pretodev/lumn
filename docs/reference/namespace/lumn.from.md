# lumn.from

> Cria um callable que retorna os itens fornecidos como fonte de dados.

## Assinatura

```lua
lumn.from(items: any[]) -> Callable
```

## Parametros

| Nome    | Tipo    | Descricao                                    |
|---------|---------|----------------------------------------------|
| `items` | `any[]` | Tabela-array de itens a serem retornados.    |

## Retorno

Um `Callable` com:
- `name` = `"lumn.from"`
- `description` = `"builtin source for tests and local development"`
- `run` = funcao que retorna a tabela `items` fornecida

## Comportamento

1. Cria um callable cujo `run` sempre retorna a mesma tabela de itens.
2. Destinado a **testes e desenvolvimento local** — em producao, o `call` usaria um callable que busca dados de uma API ou banco.
3. Pode ser passado diretamente para o campo `exec` de um `call`.

## Exemplos

```lua
-- Fonte simples com strings
return {
  flow = {
    call { exec = lumn.from({ "a", "b", "c" }) },
    set { to = function(item) return string.upper(item) end },
  }
}
```

```lua
-- Fonte com tabelas estruturadas
local orders = lumn.from({
  { id = 1, customer = "Alice", total = 150 },
  { id = 2, customer = "Bob",   total = 80 },
  { id = 3, customer = "Carol", total = 320 },
})

return {
  flow = {
    call { exec = orders },
    filter { condition = function(item) return item.total > 100 end },
  }
}
```

## Ver tambem

- [call](../primitives/call.md) — primitivo que consome o callable
- [Callable](../contracts/callable.md) — contrato do callable retornado
