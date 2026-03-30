# lumn.triggers.webhook

> Cria um trigger que dispara a execucao quando o daemon recebe uma requisicao HTTP.

## Assinatura

```lua
lumn.triggers.webhook { path = string, method? = string }
```

## Propriedades

| Nome     | Tipo     | Obrigatorio | Descricao                                         |
|----------|----------|-------------|----------------------------------------------------|
| `path`   | `string` | Sim         | Caminho do endpoint HTTP. Ex: `"/hooks/novo-pedido"`. |
| `method` | `string` | Nao         | Metodo HTTP aceito. Default: `"POST"`.              |

## Regras de validacao

- `path` deve ser unico entre todos os workflows registrados no daemon. Conflito gera erro.
- `method` aceita metodos HTTP padroes (`GET`, `POST`, `PUT`, `DELETE`, etc.).

## Comportamento

1. O daemon registra o `path` no servidor HTTP de webhooks (por padrao `localhost:6890`).
2. Ao receber uma requisicao correspondente, o daemon enfileira a execucao.
3. O response HTTP retorna `202 Accepted` com o ID da execucao.
4. O contexto do trigger fica disponivel via `lumn.trigger_data()`:
   ```lua
   {
     type    = "webhook",
     body    = { ... },      -- corpo da requisicao (parsed como table)
     headers = { ... },      -- headers da requisicao
     method  = "POST",
     path    = "/hooks/novo-pedido",
   }
   ```

## Exemplos

```lua
-- Webhook basico
lumn.triggers.webhook {
  path = "/hooks/novo-pedido",
}
```

```lua
-- Webhook com metodo especifico
lumn.triggers.webhook {
  path   = "/hooks/cancelamento",
  method = "POST",
}
```

```lua
-- Usar dados do webhook no workflow
return {
  triggers = {
    lumn.triggers.webhook { path = "/hooks/order" },
  },
  flow = {
    call {
      exec = {
        name = "from_webhook",
        run = function()
          local trigger = lumn.trigger_data()
          return trigger.body.items or {}
        end
      }
    },
    set { to = function(item) item.source = "webhook"; return item end },
  }
}
```

## Ver tambem

- [lumn.trigger_data](../namespace/lumn.trigger_data.md) — acessar o body e headers da requisicao
- [scheduler](scheduler.md) — trigger baseado em tempo
- [Trigger Data](../contracts/trigger-data.md) — formato completo do contexto
