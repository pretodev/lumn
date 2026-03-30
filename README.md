# lumn

`lumn` é um runtime de workflows em Lua, embutido em Go, com sandbox, CLI local e daemon.

O estado atual do projeto cobre:

- VM Lua embutida via `github.com/speedata/go-lua`
- sandbox para bloquear I/O e execução arbitrária
- `require` restrito a módulos locais do workflow e ao `_shared/` do workspace resolvido
- primitivos suportados: `call`, `set`, `filter` e `tap`
- estado global via `lumn.get("chave")` e `lumn.set("chave", valor)`
- triggers `manual`, `scheduler`, `webhook` e `file_watcher` no daemon
- CLI com `validate`, `run`, `start`, `stop`, `delete`, `restart`, `list`, `watch`, `logs` e `daemon`

Mais contexto de produto está em [docs/index.md](docs/index.md).

## Requisitos

- Go 1.26+

## Instalação

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

Instalar o binário do daemon:

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

Crie um `lumn.lua`:

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
  end,
}

return {
  flow = {
    call {
      exec = lumn.from(items),
      on_data = function(result)
        return result
      end,
    },
    set {
      to = function(item)
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

Valide:

```bash
lumn validate
```

Execute localmente:

```bash
lumn run
```

Suba o daemon e registre o workflow:

```bash
lumn daemon start
lumn start
lumn list
```

`lumn run` sempre escreve JSON no `stdout`. Qualquer `print(...)` do workflow vai para `stderr`.

## Entrypoint e resolução

O runtime usa estas regras:

| Situação | Entrypoint |
| --- | --- |
| `lumn run` / `lumn validate` sem alvo | `./lumn.lua` |
| `-f <pasta>` | `<pasta>/init.lua`, depois `<pasta>/lumn.lua` |
| `-f <arquivo>` | arquivo exato |
| `lumn run <selector>` | 1. daemon, 2. pasta local, 3. `<selector>.lua` |

O nome do workflow em modo standalone é inferido do contexto:

- pasta atual para `lumn run` sem argumento
- nome da pasta para `-f <pasta>`
- nome do arquivo sem extensão para `-f <arquivo>`

Em modo standalone, `version` no JSON é sempre `"latest"`.

## CLI

Comandos disponíveis:

```text
lumn validate
lumn validate -f <arquivo|pasta>

lumn run
lumn run <id|name>
lumn run -f <arquivo|pasta>

lumn start [name[:tag]] [-f <arquivo|pasta>]
lumn stop <id|name>
lumn delete <id|name>
lumn restart <id|name>
lumn list
lumn watch [id|name]
lumn logs [id|name] [--lines <n>] [--no-follow] [--since <duration>] [--level <level>] [--step <nome>]

lumn daemon start
lumn daemon stop
lumn daemon status
```

Observações:

- `lumn start` exige o daemon ativo.
- `lumn watch` e `lumn logs` já estão registrados na CLI, mas ainda retornam placeholder.
- `lumn list` mostra `ID`, `NAME`, `VERSION`, `STATUS`, `LAST RUN`, `FAILS` e `NEXT RUN`.
- seletores `<id|name>` aceitam ID completo, prefixo único de ID ou nome do workflow.
- use `lumn help <command>` ou `lumn <command> --help` para a documentação detalhada de cada comando.

## DSL mínima

Um callable é uma table Lua com este contrato:

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

- `call { exec = callable, on_data = function(result) return item end }` (`on_data` é opcional)
- `set { to = function(item) return item end }`
- `filter { condition = function(item) return boolean end }`
- `tap { exec = callable }`
- `lumn.from(items)`
- `lumn.get("chave")`
- `lumn.set("chave", valor)`

Semântica atual:

- o flow sempre inicia com batch vazio
- `call` pode aparecer em qualquer posição e substitui o batch atual
- `call.exec` roda uma vez; sem `on_data`, cada resultado bruto vira item diretamente
- se `call.exec` retornar uma table-array pura, cada posição vira um item; qualquer outro valor vira um único item
- `set.to` transforma o item atual
- `filter.condition` decide se o item continua no batch; com batch vazio, o step é ignorado
- `tap.exec` recebe uma cópia do item atual e seu retorno é descartado; com batch vazio, roda uma vez com `nil`
- ao final da execução, flows não vazios que terminam sem itens retornam status `empty`

## Workspace e `require`

Ao carregar um workflow, o engine infere o workspace subindo na árvore de diretórios até encontrar um destes marcadores:

- `lumn.lock`
- `lumn.config.lua`
- `lumn.config.*.lua`
- `_shared/`

Se nenhum marcador existir, o parent da pasta do workflow vira o workspace. O `require` pode carregar:

- módulos locais do workflow
- módulos em `<workspace>/_shared/`

Qualquer tentativa de sair desse sandbox falha com erro `ERR_SANDBOX`.

## Saída do `run`

Exemplo de sucesso:

```json
{
  "workflow": "pedidos",
  "version": "latest",
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
  "version": "latest",
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
  "version": "latest",
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
