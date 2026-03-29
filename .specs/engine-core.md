# Spec for Engine Core

branch: claude/feature/engine-core

## Summary

Implementar o nucleo atual do engine lumn com:

- VM Lua embarcada com sandbox
- DSL publica baseada em `call`, `set`, `filter` e `tap`
- estado global explicito via `lumn.get` e `lumn.set`
- DAG builder linear para a table `flow`
- executor sequencial para o subconjunto suportado
- CLI basica com `init`, `run` e `validate`
- resolucao de workspace para `_shared` e metadados do projeto

Esta entrega nao adiciona novos primitivos do roadmap. Ela apenas alinha a implementacao e a interface publica do engine-core com o contrato atual do Documento de Visao.

## Decisions

- **Backend Lua atual preservado** — o runtime continua em `github.com/speedata/go-lua`.
- **Primitivos publicos bare** — a DSL usa `call {}`, `set {}`, `filter {}` e `tap {}` como globals bare no `init.lua`.
- **`lumn` reservado para utilitarios de runtime** — nesta fase, `lumn` expõe `test_source`, `get` e `set`.
- **Sem compatibilidade com a DSL antiga** — `exec(...)`, `set(function...)`, `filter(function...)` e `tap(function...)` deixam de ser sintaxe valida.
- **Workspace com inferencia e fallback** — o engine procura `lumn.lock`, `lumn.config.lua`, `lumn.config.*.lua` ou `_shared/` em ancestrais do workflow; sem marcadores, usa o parent do workflow como workspace.
- **`lumn run` produz JSON estruturado no stdout** — o output continua sempre em JSON.

## Exit Codes

| Code | Nome | Descricao |
|------|------|-----------|
| `0` | `OK` | Execucao bem-sucedida |
| `1` | `ERR_GENERIC` | Erro generico nao categorizado |
| `2` | `ERR_SYNTAX` | Erro de sintaxe Lua no `init.lua` ou modulo requerido |
| `3` | `ERR_STRUCTURE` | Erro estrutural no workflow |
| `4` | `ERR_UNKNOWN_PRIMITIVE` | Primitivo desconhecido no `flow` |
| `5` | `ERR_INVALID_SIGNATURE` | Assinatura invalida de primitivo ou callable |
| `6` | `ERR_SANDBOX` | Acesso bloqueado pelo sandbox |
| `7` | `ERR_RUNTIME` | Erro de runtime durante execucao |
| `8` | `ERR_WORKFLOW_NOT_FOUND` | Workflow ou `init.lua` nao encontrado |
| `9` | `ERR_CALLABLE_NOT_FOUND` | Callable nao existe ou nao e resolvivel |

## Functional Requirements

### VM Lua e sandbox

- Embarcar uma VM Lua no binario Go
- Bloquear `io`, `debug`, `load`, `loadfile`, `dofile`, `os.execute`, `os.exit`, `os.remove`, `os.rename`, `os.tmpname` e `os.getenv`
- Manter `require` funcional apenas para:
  - modulos locais da pasta do workflow
  - modulos em `<workspace>/_shared/`
- Injetar o global `lumn` antes da execucao do workflow

### Workspace e resolucao do target

- `lumn run` e `lumn validate` aceitam uma pasta de workflow ou um `init.lua`
- O engine resolve primeiro a pasta do workflow
- Depois disso, infere o workspace subindo na arvore e procurando:
  - `lumn.lock`
  - `lumn.config.lua`
  - `lumn.config.*.lua`
  - `_shared/`
- Se nada for encontrado, o workspace vira o parent da pasta do workflow

### Callables

Um callable continua sendo uma table Lua com:

```lua
{
  name = "meu_callable",
  description = "opcional",
  run = function(input)
    return input
  end
}
```

- `name` e obrigatorio
- `run` e obrigatorio e precisa ser funcao
- `description` e opcional

#### Callable builtin: `lumn.test_source`

```lua
lumn.test_source(items)
```

- Recebe uma table-array
- Retorna um callable valido para `call.exec`
- E usado para desenvolvimento local e testes

### DSL publica

Primitivos suportados:

- **`call { exec = callable, on_data = function(result) return item end }`**
  - `call` e o unico node que cria a lista inicial de itens
  - `exec` roda uma vez e deve retornar uma table-array
  - `on_data` recebe cada resultado bruto e deve retornar o item inicial
- **`set { to = function(item) return item end }`**
  - transformacao pura do item
  - nao recebe `res` nem `ctx`
- **`filter { condition = function(item) return boolean end }`**
  - remove itens quando retorna `false`
- **`tap { exec = callable }`**
  - executa um callable por item
  - recebe uma copia do item atual
  - retorno e descartado

Funcoes utilitarias do runtime:

- `lumn.test_source(items)`
- `lumn.get("chave")`
- `lumn.set("chave", valor)`

### Estado global da execucao

- O estado compartilhado da execucao nao e passado como argumento para callbacks da DSL
- O acesso acontece apenas via:
  - `lumn.set("chave", valor)`
  - `lumn.get("chave")`
- O estado existe somente durante `run`

### DAG Builder

- Parsear a table retornada por `init.lua`
- Validar campos obrigatorios: `id`, `version`, `flow`
- Aceitar apenas `call`, `set`, `filter` e `tap`
- Em `flow` nao vazio:
  - o primeiro node precisa ser `call`
  - nenhum node posterior pode ser `call`
- Rejeitar sintaxe antiga e primitivos desconhecidos com erro claro e posicao

### Executor Sequencial

- Processar os itens sequencialmente
- `call.exec` produz o lote bruto
- `call.on_data` transforma cada resultado bruto em item
- `set.to` substitui o item atual
- `filter.condition` decide se o item continua
- `tap.exec` roda com uma copia do item atual e nao altera o fluxo
- Se o batch ficar vazio em qualquer ponto, o workflow encerra com:
  - exit code `0`
  - status `"empty"`

### CLI Basica

- **`lumn init <nome>`** — cria `<nome>/init.lua` com scaffold na DSL atual
- **`lumn run <workflow|init.lua>`** — carrega o workflow, valida e executa
- **`lumn validate <workflow|init.lua>`** — valida sintaxe Lua, estrutura e assinaturas

## Possible Edge Cases

- `init.lua` nao retorna table
- `flow` nao e table
- `call` sem `exec`
- `call` sem `on_data`
- `set` sem `to`
- `filter` sem `condition`
- `tap` sem `exec`
- `call.exec` retorna valor que nao e table
- `call.on_data` retorna `nil`
- `set.to` retorna `nil`
- `filter.condition` retorna valor que nao e boolean
- `require` tenta escapar do sandbox
- `lumn.get` ou `lumn.set` e chamado fora de uma execucao

## Acceptance Criteria

- Um workflow com `call -> set -> filter -> tap` executa via `lumn run` e produz JSON valido
- `lumn init meu-workflow` cria um scaffold valido para `lumn validate`
- `lumn validate` rejeita a DSL antiga e retorna os exit codes corretos
- `lumn.get` e `lumn.set` funcionam entre steps
- `require` funciona para modulos locais e para `_shared/` do workspace resolvido
- Um workflow cujo batch fica vazio termina com status `"empty"`
- Cada exit code definido possui pelo menos um teste

## Testing Guidelines

```gherkin
Scenario: Executar workflow simples com a DSL atual
  Given um workflow "pedidos" com call(lumn.test_source) retornando 3 itens
  And set que adiciona campo
  And filter que remove 1 item
  And tap que loga o item
  When o desenvolvedor executa "lumn run pedidos"
  Then o stdout contem JSON valido com items_in=3, items_out=2 e status="ok"
  And o exit code e 0

Scenario: Pipeline fica vazia na fonte
  Given um workflow cujo call usa lumn.test_source({})
  When o desenvolvedor executa "lumn run"
  Then o stdout contem status="empty"
  And o exit code e 0

Scenario: DSL antiga e rejeitada
  Given um workflow cujo flow contem "exec(lumn.test_source(...))"
  When o desenvolvedor executa "lumn validate"
  Then o exit code e 4
  And stderr indica primitive desconhecido

Scenario: Workspace e inferido por marcador
  Given um projeto com lumn.lock na raiz e _shared/helper.lua
  And um workflow em subdiretorio usa require("helper")
  When o desenvolvedor executa "lumn validate"
  Then a validacao passa

Scenario: Fluxo nao comeca com call
  Given um workflow cujo primeiro node e set
  When o desenvolvedor executa "lumn validate"
  Then o exit code e 3
  And stderr informa que flow nao vazio deve comecar com call
```
