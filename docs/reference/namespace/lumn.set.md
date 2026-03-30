# lumn.set

> Armazena um valor no estado de execucao.

## Assinatura

```lua
lumn.set(key: string, value: any)
```

## Parametros

| Nome    | Tipo     | Descricao                                         |
|---------|----------|----------------------------------------------------|
| `key`   | `string` | Chave para armazenar o valor.                      |
| `value` | `any`    | Valor a armazenar. Passar `nil` remove a chave.    |

## Retorno

Nenhum.

## Comportamento

1. Armazena um valor no estado de execucao compartilhado entre todos os steps.
2. Disponivel **apenas durante a execucao** — dentro de funcoes `run`, `to`, `condition` ou `on_data`.
3. Chamadas fora do contexto de execucao geram `ERR_RUNTIME`.
4. Passar `nil` como `value` remove a entrada da chave.
5. O estado **nao persiste** entre execucoes diferentes do mesmo workflow.

## Exemplos

```lua
-- Inicializar estado no primeiro step
tap {
  exec = {
    name = "init",
    run = function()
      lumn.set("started_at", os.time())
      lumn.set("error_count", 0)
    end
  }
}
```

```lua
-- Acumular informacao entre steps
set {
  to = function(item)
    if item.status == "error" then
      local errors = lumn.get("error_count") or 0
      lumn.set("error_count", errors + 1)
    end
    return item
  end
}
```

## Erros comuns

| Codigo        | Situacao                                                    |
|---------------|-------------------------------------------------------------|
| `ERR_RUNTIME` | Chamada fora do contexto de execucao (ex: no escopo global). |

## Ver tambem

- [lumn.get](lumn.get.md) — recupera valores do estado de execucao
