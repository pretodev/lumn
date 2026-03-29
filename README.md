# lumn

`lumn` é um engine de workflows em Lua, embutido em Go, com sandbox e uma DSL mínima baseada em pipeline.

O estado atual do projeto cobre a fase `engine-core`, alinhada ao subconjunto já implementado do Documento de Visão:

- VM Lua embutida via `github.com/speedata/go-lua`
- sandbox para bloquear I/O e execução arbitrária
- `require` restrito a módulos locais do workflow e ao `_shared/` do workspace resolvido
- primitivos suportados: `call`, `set`, `filter` e `tap`
- estado global da execução via `lumn.get("chave")` e `lumn.set("chave", valor)`
- callable builtin `lumn.test_source(items)`
- CLI com `init`, `validate` e `run`
- saída estruturada em JSON no `stdout`

Mais contexto de produto está em [docs/index.md](docs/index.md). A especificação implementada nesta fase está em [.specs/engine-core.md](.specs/engine-core.md).

## Requisitos

- Go 1.26+

## Instalacao

Instalar a CLI localmente a partir do checkout:

```bash
go install ./cmd/lumn
```

Ou via `make`:

```bash
make install
```

Build local sem instalar:

```bash
make build
```

Instalar o binario do daemon placeholder:

```bash
make install-daemon
```

## Desenvolvimento

Rodar a suite:

```bash
make test
```

Também funciona:

```bash
go test ./...
```

## Quickstart

Crie um workflow:

```bash
lumn init pedidos
```

Valide:

```bash
lumn validate pedidos
```

Execute:

```bash
lumn run pedidos
```

`lumn run` sempre escreve JSON no `stdout`. Qualquer `print(...)` do workflow vai para `stderr`.

## Exemplo de workflow

```lua
local items = {
  { id = 1, nome = "Item A", valor = 100 },
  { id = 2, nome = "Item B", valor = 50 },
  { id = 3, nome = "Item C", valor = 200 },
}

local log_item = {
  name = "log_item",
  run = function(input)
    print(input.nome .. " aprovado")
  end
}

return {
  id = "pedidos",
  version = "1.0.0",
  flow = {
    call {
      exec = lumn.test_source(items),
      on_data = function(result)
        return result
      end,
    },
    set {
      to = function(item)
        lumn.set("ultimo_item_id", item.id)
        item.ultimo_item_id = lumn.get("ultimo_item_id")
        item.processado = true
        return item
      end,
    },
    filter {
      condition = function(item)
        return item.valor > 80
      end,
    },
    tap {
      exec = log_item,
    },
  }
}
```

## DSL minima

Um callable e uma table Lua com este contrato:

```lua
{
  name = "meu_callable",
  description = "opcional",
  run = function(input)
    return input
  end
}
```

Primitivos suportados nesta fase:

- `call { exec = callable, on_data = function(result) return item end }`
- `set { to = function(item) return item end }`
- `filter { condition = function(item) return boolean end }`
- `tap { exec = callable }`
- `lumn.test_source(items)`
- `lumn.get("chave")`
- `lumn.set("chave", valor)`

Semantica atual:

- `call` e o unico primitivo que cria a lista inicial de itens
- `call.exec` roda uma vez e `call.on_data` transforma cada resultado bruto em item
- `set.to` transforma o item atual sem receber `res` ou `ctx`
- `filter.condition` decide se o item continua no batch
- `tap.exec` recebe uma copia do item atual e seu retorno e descartado
- quando o batch fica vazio durante a execucao, o workflow encerra com status `empty`

## Workspace e `require`

`lumn run` e `lumn validate` continuam aceitando uma pasta de workflow ou um `init.lua`.

Ao carregar um workflow, o engine infere o workspace subindo na arvore de diretorios ate encontrar um destes marcadores:

- `lumn.lock`
- `lumn.config.lua`
- `lumn.config.*.lua`
- `_shared/`

Se nenhum marcador existir, o parent da pasta do workflow vira o workspace. O `require` pode carregar:

- modulos locais do workflow
- modulos em `<workspace>/_shared/`

Qualquer tentativa de sair desse sandbox falha com erro `ERR_SANDBOX`.

## Saida do run

Exemplo de sucesso:

```json
{
  "workflow": "pedidos",
  "version": "1.0.0",
  "status": "ok",
  "items_in": 3,
  "items_out": 2,
  "errors": [],
  "duration_ms": 1
}
```

Exemplo de batch vazio:

```json
{
  "workflow": "pedidos",
  "version": "1.0.0",
  "status": "empty",
  "items_in": 0,
  "items_out": 0,
  "errors": [],
  "duration_ms": 1
}
```

Exemplo de erro:

```json
{
  "workflow": "pedidos",
  "version": "1.0.0",
  "status": "error",
  "items_in": 3,
  "items_out": 0,
  "errors": [
    {
      "type": "runtime",
      "primitive": "set",
      "position": 2,
      "message": "set.to must return item, got nil"
    }
  ],
  "duration_ms": 1
}
```

## Exit codes

| Code | Nome |
| --- | --- |
| `0` | `OK` |
| `1` | `ERR_GENERIC` |
| `2` | `ERR_SYNTAX` |
| `3` | `ERR_STRUCTURE` |
| `4` | `ERR_UNKNOWN_PRIMITIVE` |
| `5` | `ERR_INVALID_SIGNATURE` |
| `6` | `ERR_SANDBOX` |
| `7` | `ERR_RUNTIME` |
| `8` | `ERR_WORKFLOW_NOT_FOUND` |
| `9` | `ERR_CALLABLE_NOT_FOUND` |

## Estrutura atual

```text
cmd/lumn      CLI da fase 0
cmd/lumnd     daemon placeholder
internal/lua  runtime Lua e sandbox
internal/dag  parse e validacao do workflow
internal/executor
internal/engine
pkg/errkind
pkg/primitive
```

## Limitacoes desta fase

- `lumnd` ainda nao implementa o runtime de daemon
- o executor e sequencial e fail-fast
- o DAG atual e linear; `pipe`, `once`, `distinct`, `branch` e `parallel` ainda nao existem
- nao ha plugins externos, triggers, persistencia ou UI nesta entrega
