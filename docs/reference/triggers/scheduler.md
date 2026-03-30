# lumn.triggers.scheduler

> Cria um trigger de agendamento por intervalo ou expressao cron.

## Assinatura

```lua
lumn.triggers.scheduler { interval = string }
lumn.triggers.scheduler { cron = string, timezone? = string }
```

## Propriedades

| Nome       | Tipo     | Obrigatorio         | Descricao                                            |
|------------|----------|---------------------|------------------------------------------------------|
| `interval` | `string` | Sim (se sem `cron`) | Intervalo entre execucoes. Sufixos: `"s"`, `"m"`, `"h"`. |
| `cron`     | `string` | Sim (se sem `interval`) | Expressao cron de 5 campos.                        |
| `timezone` | `string` | Nao                 | Timezone IANA para cron. Ex: `"America/Sao_Paulo"`, `"UTC"`. |

## Regras de validacao

- `interval` e `cron` sao **mutuamente exclusivos**. Fornecer ambos gera erro.
- `interval` aceita sufixos: `s` (segundos), `m` (minutos), `h` (horas). Ex: `"15m"`, `"1h"`, `"30s"`.
- `cron` segue o formato padrao de 5 campos: minuto, hora, dia do mes, mes, dia da semana.
- `timezone` so e valido com `cron`. Se omitido, usa o timezone local do sistema.

## Comportamento

1. O daemon agenda a proxima execucao com base no intervalo ou expressao cron.
2. Quando o horario agendado e atingido, o daemon enfileira a execucao do workflow.
3. O proximo horario agendado e persistido em SQLite e restaurado em caso de restart do daemon.
4. O contexto do trigger fica disponivel via `lumn.trigger_data()`:
   ```lua
   { type = "scheduler", scheduled_at = "2025-03-15T09:00:00Z", fired_at = "2025-03-15T09:00:01Z" }
   ```

## Exemplos

```lua
-- A cada 15 minutos
lumn.triggers.scheduler { interval = "15m" }
```

```lua
-- A cada hora
lumn.triggers.scheduler { interval = "1h" }
```

```lua
-- Dias uteis as 9h (Sao Paulo)
lumn.triggers.scheduler {
  cron     = "0 9 * * MON-FRI",
  timezone = "America/Sao_Paulo",
}
```

```lua
-- Toda meia-noite UTC
lumn.triggers.scheduler {
  cron     = "0 0 * * *",
  timezone = "UTC",
}
```

## Ver tambem

- [lumn.trigger_data](../namespace/lumn.trigger_data.md) — acessar dados do trigger na execucao
- [webhook](webhook.md) — trigger baseado em requisicao HTTP
- [Workflow](../contracts/workflow.md) — onde declarar triggers
