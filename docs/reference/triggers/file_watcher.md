# lumn.triggers.file_watcher

> Cria um trigger que dispara a execucao quando eventos de filesystem sao detectados.

## Assinatura

```lua
lumn.triggers.file_watcher {
  path     = string,
  pattern? = string,
  event?   = string,
  debounce? = string,
}
```

## Propriedades

| Nome       | Tipo     | Obrigatorio | Default    | Descricao                                          |
|------------|----------|-------------|------------|-----------------------------------------------------|
| `path`     | `string` | Sim         | —          | Caminho absoluto do diretorio a monitorar.          |
| `pattern`  | `string` | Nao         | —          | Filtro glob para nomes de arquivo. Ex: `"*.csv"`.   |
| `event`    | `string` | Nao         | `"any"`    | Tipo de evento: `"create"`, `"modify"`, `"delete"`, `"any"`. |
| `debounce` | `string` | Nao         | `"500ms"`  | Intervalo para agrupar eventos rapidos. Sufixos: `"ms"`, `"s"`. |

## Comportamento

1. O daemon monitora o diretorio usando notificacoes do filesystem (fsnotify).
2. Ao detectar um evento correspondente, aguarda a janela de debounce para agrupar rajadas.
3. Apos o debounce, enfileira a execucao do workflow.
4. O contexto do trigger fica disponivel via `lumn.trigger_data()`:
   ```lua
   {
     type  = "file_watcher",
     file  = "dados.csv",
     event = "create",
     path  = "/data/importacoes",
   }
   ```

## Exemplos

```lua
-- Monitorar criacao de CSVs
lumn.triggers.file_watcher {
  path    = "/data/importacoes",
  pattern = "*.csv",
  event   = "create",
}
```

```lua
-- Monitorar qualquer alteracao com debounce maior
lumn.triggers.file_watcher {
  path     = "/data/importacoes",
  pattern  = "*.csv",
  event    = "create",
  debounce = "2s",
}
```

```lua
-- Workflow completo com file_watcher
return {
  triggers = {
    lumn.triggers.file_watcher {
      path    = "/data/exports",
      pattern = "*.json",
      event   = "create",
    },
  },
  flow = {
    call {
      exec = {
        name = "read_file_info",
        run = function()
          local trigger = lumn.trigger_data()
          return { { file = trigger.file, dir = trigger.path } }
        end
      }
    },
    tap {
      exec = {
        name = "notify",
        run = function(item)
          print("Novo arquivo:", item.file)
        end
      }
    },
  }
}
```

## Ver tambem

- [lumn.trigger_data](../namespace/lumn.trigger_data.md) — acessar detalhes do evento
- [scheduler](scheduler.md) — trigger baseado em tempo
- [Trigger Data](../contracts/trigger-data.md) — formato completo do contexto
