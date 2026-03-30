# tap

> Executa um efeito colateral sem alterar o batch.

## Assinatura

```lua
tap { exec = Callable }
```

## Propriedades

| Nome   | Tipo       | Obrigatorio | Descricao                                     |
|--------|------------|-------------|------------------------------------------------|
| `exec` | `Callable` | Sim         | Callable executado como efeito colateral.      |

## Comportamento

1. Para cada item do batch, cria uma **copia** do item e invoca `exec.run(copia, state)`.
2. O valor retornado pelo callable e **descartado** — o batch original permanece inalterado.
3. Se o batch estiver vazio, o callable e invocado **uma vez** com `nil` como input.
4. Erros dentro do callable propagam normalmente e interrompem a execucao.

## Exemplos

```lua
-- Log de cada item processado
tap {
  exec = {
    name = "log_items",
    run = function(item)
      if item then
        print("Processando:", item.id)
      else
        print("Batch vazio")
      end
    end
  }
}
```

```lua
-- Enviar notificacao sem alterar o pipeline
tap {
  exec = notify_slack,
}
```

```lua
-- Inicializar estado antes do processamento
tap {
  exec = {
    name = "init",
    run = function()
      lumn.set("started_at", os.time())
    end
  }
}
```

## Erros comuns

| Codigo                   | Situacao                                               |
|--------------------------|--------------------------------------------------------|
| `ERR_INVALID_SIGNATURE`  | `exec` nao e um callable valido (falta `name` ou `run`). |
| `ERR_CALLABLE_NOT_FOUND` | O simbolo referenciado em `exec` nao foi encontrado.    |

## Ver tambem

- [call](call.md) — similar, mas os resultados entram no batch
- [Callable](../contracts/callable.md) — contrato que `exec` deve satisfazer
