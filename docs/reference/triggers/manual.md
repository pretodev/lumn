# lumn.triggers.manual

> Cria um trigger para execucao sob demanda via CLI.

## Assinatura

```lua
lumn.triggers.manual {}
```

## Propriedades

Nenhuma. Aceita uma tabela vazia.

## Comportamento

1. O workflow fica registrado e carregado no daemon, pronto para execucao.
2. Nao e disparado automaticamente — requer invocacao explicita via `lumn exec <workflow>`.
3. Se o campo `triggers` do workflow estiver ausente ou vazio, um trigger manual e **assumido implicitamente**.
4. O contexto do trigger retorna:
   ```lua
   { type = "manual" }
   ```

## Exemplos

```lua
-- Trigger manual explicito
return {
  triggers = {
    lumn.triggers.manual {},
  },
  flow = {
    call { exec = lumn.from({ "item1", "item2" }) },
  }
}
```

```lua
-- Trigger manual implicito (sem campo triggers)
return {
  flow = {
    call { exec = lumn.from({ "item1", "item2" }) },
  }
}
```

```lua
-- Combinado com outros triggers
return {
  triggers = {
    lumn.triggers.scheduler { interval = "1h" },
    lumn.triggers.manual {},
  },
  flow = {
    call { exec = process_orders },
  }
}
```

## CLI

```bash
# Disparar execucao manual de um workflow registrado
lumn exec <workflow-id>
```

## Ver tambem

- [scheduler](scheduler.md) — trigger automatico baseado em tempo
- [webhook](webhook.md) — trigger baseado em requisicao HTTP
- [Workflow](../contracts/workflow.md) — estrutura completa do workflow
