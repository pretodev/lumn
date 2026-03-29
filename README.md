# lumn

`lumn` e um engine de workflows em Lua 5.4, embutido em Go, com sandbox e uma DSL minima baseada em pipeline.

O estado atual do projeto cobre a fase `engine-core`:

- VM Lua 5.4 embutida via `github.com/speedata/go-lua`
- sandbox para bloquear I/O e execucao arbitraria
- `require` restrito a modulos locais do workflow e `_shared/`
- primitivos `exec`, `set`, `filter` e `tap`
- callable builtin `lumn.test_source`
- CLI com `init`, `validate` e `run`
- saida estruturada em JSON no `stdout`

Mais contexto de produto esta em [docs/index.md](docs/index.md). A especificacao implementada nesta fase esta em [.specs/engine-core.md](.specs/engine-core.md).

## Requisitos

- Go 1.26+

## Instalacao

Instalar a CLI localmente a partir do checkout:

```bash
go install .
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

Tambem funciona:

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

return {
  id = "pedidos",
  version = "1.0.0",
  flow = {
    exec(lumn.test_source(items)),
    set(function(res, item, ctx)
      item.processado = true
      return item
    end),
    filter(function(item, ctx)
      return item.valor > 80
    end),
    tap(function(item, ctx)
      print(item.nome .. " aprovado")
    end),
  }
}
```

## DSL minima

Um callable e uma table Lua com este contrato:

```lua
{
  name = "meu_callable",
  description = "opcional",
  run = function(input, ctx)
    return {}
  end
}
```

Primitivos suportados nesta fase:

- `exec(callable)` ou `lumn.exec(callable)`
- `set(fn)` ou `lumn.set(fn)`
- `filter(fn)` ou `lumn.filter(fn)`
- `tap(fn)` ou `lumn.tap(fn)`
- `lumn.test_source(items)`

Semantica atual:

- o primeiro `exec` produz a lista inicial de itens
- `exec` posteriores recebem o `item` atual como input
- o retorno de `exec` vira `res` no proximo `set`
- `ctx` e criado internamente e compartilhado ao longo da execucao
- lista vazia encerra naturalmente com status `ok`

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
      "message": "set must return item, got nil"
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
- o DAG atual e linear; primitives avancados ainda nao existem
- nao ha plugins externos, triggers, persistencia ou UI nesta entrega
