# Referencia da DSL Lua

Referencia completa da API Lua do lumn. Cobre primitivos, namespace global, triggers e contratos.

## Primitivos

Funcoes globais que compõem o pipeline de execucao dentro do campo `flow` de um workflow.

| Primitivo              | Descricao                                              |
|------------------------|--------------------------------------------------------|
| [call](primitives/call.md)     | Invoca um callable e injeta os resultados no batch.   |
| [set](primitives/set.md)       | Transforma cada item do batch.                        |
| [filter](primitives/filter.md) | Remove itens que nao satisfazem uma condicao.         |
| [tap](primitives/tap.md)       | Executa efeito colateral sem alterar o batch.         |

## Namespace `lumn`

Funcoes utilitarias e acesso ao estado de execucao.

| Funcao                                               | Descricao                                        |
|------------------------------------------------------|--------------------------------------------------|
| [lumn.from](namespace/lumn.from.md)                  | Cria callable de teste com itens estaticos.      |
| [lumn.get](namespace/lumn.get.md)                    | Recupera valor do estado de execucao.            |
| [lumn.set](namespace/lumn.set.md)                    | Armazena valor no estado de execucao.            |
| [lumn.trigger_data](namespace/lumn.trigger_data.md)  | Retorna contexto do trigger que disparou.        |

## Triggers

Construtores de triggers disponiveis em `lumn.triggers.*`. Definem quando e como o daemon dispara o workflow.

| Trigger                                                  | Descricao                                          |
|----------------------------------------------------------|----------------------------------------------------|
| [lumn.triggers.scheduler](triggers/scheduler.md)         | Agendamento por intervalo ou cron.                 |
| [lumn.triggers.webhook](triggers/webhook.md)             | Disparo via requisicao HTTP.                       |
| [lumn.triggers.file_watcher](triggers/file_watcher.md)   | Monitoramento de eventos no filesystem.            |
| [lumn.triggers.manual](triggers/manual.md)               | Execucao sob demanda via CLI.                      |

## Contratos

Definicoes das estruturas de dados usadas na DSL.

| Contrato                                        | Descricao                                               |
|-------------------------------------------------|---------------------------------------------------------|
| [Callable](contracts/callable.md)               | Interface de uma unidade de trabalho executavel.        |
| [Workflow](contracts/workflow.md)                | Estrutura da tabela retornada pelo arquivo de entrada.  |
| [Trigger Data](contracts/trigger-data.md)       | Formato dos dados retornados por `lumn.trigger_data()`. |

## Exemplo rapido

```lua
local orders = lumn.from({
  { id = 1, customer = "Alice", total = 150 },
  { id = 2, customer = "Bob",   total = 80 },
  { id = 3, customer = "Carol", total = 320 },
})

return {
  triggers = {
    lumn.triggers.scheduler { interval = "15m" },
    lumn.triggers.manual {},
  },

  flow = {
    call { exec = orders },

    filter {
      condition = function(item)
        return item.total > 100
      end
    },

    set {
      to = function(item)
        item.label = string.format("%s: R$%.2f", item.customer, item.total)
        return item
      end
    },

    tap {
      exec = {
        name = "log",
        run = function(item)
          print(item.label)
        end
      }
    },
  }
}
```

## Integracao com editores

O projeto inclui type stubs em [types/](../../types/) no formato LuaCATS, compativeis com o [lua-language-server](https://luals.github.io). Isso habilita autocomplete, hover docs e go-to-definition em qualquer editor com suporte a LSP (NeoVim, VSCode, etc.).

A configuracao esta em [.luarc.json](../../.luarc.json) na raiz do projeto.

### NeoVim

Para projetos que usam lumn, configure o `lua_ls` apontando para os stubs:

```lua
require("lspconfig").lua_ls.setup({
  settings = {
    Lua = {
      runtime = { version = "Lua 5.3" },
      workspace = {
        library = { "/caminho/para/lumn/types" },
      },
    },
  },
})
```

### VSCode

Instale a extensao [Lua](https://marketplace.visualstudio.com/items?itemName=sumneko.lua) e adicione um `.luarc.json` na raiz do workspace:

```json
{
  "runtime.version": "Lua 5.3",
  "workspace.library": ["/caminho/para/lumn/types"]
}
```
