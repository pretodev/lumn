# Spec for Engine Core

branch: claude/feature/engine-core

## Summary

Implementar o núcleo do engine lumn: uma VM Lua 5.4 embarcada com sandbox de segurança, o global `lumn` expondo os primitivos fundamentais da DSL (`exec`, `set`, `filter`, `tap`), um DAG builder que monta o pipeline a partir da table `flow`, um executor sequencial que processa itens através do pipeline, e uma CLI básica com os comandos `init`, `run` e `validate`.

Esta é a fundação sobre a qual todo o restante da plataforma será construído. Ao final desta entrega, um desenvolvedor deve conseguir criar um projeto, escrever um workflow com callables reais, validar a estrutura e executar o pipeline localmente — incluindo um callable de teste embutido para validação rápida.

## Decisions

- **Lua 5.4** — a VM embarcada deve ser compatível com Lua 5.4. Avaliar bibliotecas Go que suportem esta versão (ex: `lua54` bindings ou alternativas). `gopher-lua` é Lua 5.1 e portanto **não** é elegível.
- **`ctx` não é inicializável via CLI** — o contexto (`ctx`) é criado internamente pelo engine como table vazia e compartilhado ao longo da execução. Não há flags de CLI para pré-popular `ctx`.
- **`lumn run` produz JSON estruturado no stdout** — o output é sempre JSON, facilitando composição com outras ferramentas e parsing programático.
- **Exit codes específicos por tipo de erro** — cada categoria de falha tem um exit code próprio (ver seção Exit Codes).
- **`exec` recebe callables reais** — desde a primeira iteração, `exec` aceita callables concretos (não apenas funções Lua puras). Um callable é uma table Lua com um campo `run` (função) e metadados opcionais (`name`, `description`). Isso permite composição e reutilização desde o início.

## Exit Codes

| Code | Nome | Descrição |
|------|------|-----------|
| `0` | `OK` | Execução bem-sucedida |
| `1` | `ERR_GENERIC` | Erro genérico não categorizado |
| `2` | `ERR_SYNTAX` | Erro de sintaxe Lua no `init.lua` ou módulo requerido |
| `3` | `ERR_STRUCTURE` | Erro de estrutura do workflow: campos obrigatórios ausentes (`id`, `version`, `flow`) ou tipos incorretos |
| `4` | `ERR_UNKNOWN_PRIMITIVE` | Primitivo desconhecido encontrado no `flow` |
| `5` | `ERR_INVALID_SIGNATURE` | Assinatura inválida em primitivo (ex: `set` sem função, `exec` sem callable) |
| `6` | `ERR_SANDBOX` | Tentativa de acesso a função bloqueada pelo sandbox (`os`, `io`, `debug`, etc.) |
| `7` | `ERR_RUNTIME` | Erro de runtime durante execução de callable ou função no pipeline |
| `8` | `ERR_WORKFLOW_NOT_FOUND` | Workflow ou `init.lua` não encontrado no path indicado |
| `9` | `ERR_CALLABLE_NOT_FOUND` | Callable referenciado não existe ou não é resolvível |

## Functional Requirements

### VM Lua 5.4 Embarcada

- Embarcar uma VM Lua 5.4 no binário Go
- Aplicar sandbox desde a primeira iteração: remover acesso a `os`, `io`, `loadfile`, `dofile`, `debug` e qualquer função que permita I/O ou execução arbitrária
- Manter `require` funcional apenas para módulos locais do projeto (resolução relativa ao diretório do workflow)
- Injetar o global `lumn` no ambiente Lua antes da execução do workflow

### Callables

Um callable é a unidade de execução do lumn. É uma table Lua com a seguinte estrutura:

```lua
{
  name = "meu_callable",          -- string, obrigatório
  description = "faz algo útil",  -- string, opcional
  run = function(input, ctx)      -- function, obrigatório
    -- lógica do callable
    return resultado
  end
}
```

- O campo `run` é obrigatório e deve ser uma função
- O campo `name` é obrigatório e deve ser uma string
- O campo `description` é opcional
- Callables são passados diretamente ao `exec`

#### Callable de Teste Embutido: `lumn.test_source`

O engine disponibiliza um callable de teste embutido para facilitar validação e desenvolvimento:

```lua
lumn.test_source(items)
```

- Recebe uma table-array de itens e retorna um callable válido
- Útil para testar pipelines sem depender de fontes externas
- Exemplo de uso:

```lua
local items = {
  { id = 1, nome = "Item A", valor = 100 },
  { id = 2, nome = "Item B", valor = 50 },
  { id = 3, nome = "Item C", valor = 200 },
}

return {
  id = "teste-pipeline",
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

### Global `lumn` com Primitivos da DSL

- Expor os quatro primitivos fundamentais como funções globais acessíveis no contexto do `flow`:
  - **`exec(callable)`** — executa um callable (table com campo `run`); o callable recebe `(input, ctx)` onde `input` é `nil` para o primeiro exec ou o resultado do anterior; o retorno do `run` fica disponível como `res` no próximo `set` e, se for o primeiro `exec` do pipeline, deve retornar uma table-array que será a lista de itens a processar
  - **`set(fn(res, item, ctx) -> item)`** — transforma o item atual; recebe o resultado do `exec` anterior (ou `nil`), o item e o contexto global
  - **`filter(fn(item, ctx) -> bool)`** — remove itens onde a função retorna `false`; item não é mutado
  - **`tap(fn(item, ctx))`** — executa efeito colateral puro; retorno é descartado, item nunca é mutado
- Quando a lista de itens fica vazia em qualquer ponto do pipeline, o workflow encerra naturalmente sem erro (exit code `0`)

### DAG Builder

- Parsear a table retornada pelo `init.lua` do workflow
- Validar a presença dos campos obrigatórios: `id` (string), `version` (string), `flow` (table)
- Construir uma representação interna do pipeline a partir da sequência de primitivos em `flow`
- Rejeitar primitivos desconhecidos com erro claro indicando nome e posição (exit code `4`)

### Executor Sequencial

- Processar itens um a um, na ordem em que aparecem na lista
- Manter o contrato de `res`: o retorno de `exec` é passado ao próximo `set`; se não houver `exec` anterior, `res` é `nil`
- Manter `ctx` como table vazia criada internamente, compartilhada entre todos os itens de uma execução
- O primeiro `exec` do pipeline é o responsável por produzir a lista inicial de itens (retorno do `run` do callable deve ser table-array)
- Output no stdout em JSON estruturado:

```json
{
  "workflow": "teste-pipeline",
  "version": "1.0.0",
  "status": "ok",
  "items_in": 3,
  "items_out": 2,
  "errors": [],
  "duration_ms": 12
}
```

Em caso de erro:

```json
{
  "workflow": "teste-pipeline",
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
  "duration_ms": 5
}
```

### CLI Básica

- **`lumn init <nome>`** — cria o esqueleto mínimo: uma pasta `<nome>/` com um arquivo `init.lua` contendo a estrutura base de um workflow usando `lumn.test_source` como callable inicial
- **`lumn run <workflow>`** — carrega o `init.lua` do workflow indicado, monta o pipeline via DAG builder e executa com o executor sequencial; output é JSON estruturado no stdout
- **`lumn validate <workflow>`** — validação completa: verifica sintaxe Lua (exit `2`), parseia a table retornada, valida campos obrigatórios (exit `3`), verifica que todos os primitivos no `flow` são conhecidos (exit `4`) e que as assinaturas estão corretas (exit `5`)

## Possible Edge Cases

- Workflow cujo `init.lua` não retorna uma table (ex: retorna `nil` ou string) — exit `3`
- Primitivo `set` que não retorna o item (retorno `nil`) — exit `7` com erro claro no JSON
- Callable passado a `exec` sem campo `run` ou sem campo `name` — exit `5`
- Callable cujo `run` lança erro Lua — exit `7` com contexto (nome do callable, posição no pipeline)
- Workflow com `flow` vazio (table sem primitivos) — exit `0`, JSON reporta 0 itens processados
- `require` tentando acessar path fora do diretório do projeto — exit `6`
- `init.lua` que tenta acessar `os.execute` ou `io.open` — exit `6` antes de qualquer side effect
- `lumn run` em diretório que não contém workflow válido — exit `8`
- `lumn validate` em arquivo com sintaxe Lua inválida — exit `2` com linha e coluna do erro
- Callable referenciado que não existe — exit `9`

## Acceptance Criteria

- Um workflow com `exec(lumn.test_source(items))` → `set` → `filter` → `tap` executa via `lumn run` e produz JSON válido no stdout com contagem correta de itens
- `lumn init meu-workflow` cria `meu-workflow/init.lua` com template usando `lumn.test_source` e é validável com `lumn validate`
- `lumn validate` rejeita workflows com erros e retorna o exit code específico para cada tipo
- A VM Lua bloqueia acesso a `os`, `io`, `loadfile`, `dofile` e `debug` — exit `6`
- `require` funciona para módulos locais do projeto e falha para paths externos — exit `6`
- Pipeline com lista vazia encerra com exit `0` e JSON reportando 0 itens
- Erros de runtime em callables são capturados e reportados no JSON com contexto — exit `7`
- O binário `lumn` é compilável como single binary Go sem dependências externas
- Cada exit code definido é testado por pelo menos um cenário

## Testing Guidelines

```gherkin
Scenario: Executar workflow simples com pipeline completo
  Given um workflow "pedidos" com exec(lumn.test_source) retornando 3 itens, set que adiciona campo, filter que remove 1 item e tap que loga
  When o desenvolvedor executa "lumn run pedidos"
  Then o stdout contém JSON válido com items_in=3, items_out=2, status="ok"
  And o exit code é 0

Scenario: Inicializar projeto com esqueleto mínimo
  Given um diretório vazio
  When o desenvolvedor executa "lumn init meu-workflow"
  Then a pasta "meu-workflow/" é criada contendo "init.lua" com lumn.test_source
  And "lumn validate meu-workflow" passa com exit code 0

Scenario: Validação rejeita workflow com campo obrigatório ausente
  Given um workflow cujo init.lua retorna table sem campo "id"
  When o desenvolvedor executa "lumn validate" no workflow
  Then o exit code é 3 (ERR_STRUCTURE)
  And stderr contém mensagem indicando que "id" é obrigatório

Scenario: Sandbox bloqueia acesso a funções perigosas
  Given um workflow cujo init.lua contém chamada a "os.execute('rm -rf /')"
  When o desenvolvedor executa "lumn run" no workflow
  Then o exit code é 6 (ERR_SANDBOX)
  And o JSON de output contém error type "sandbox"

Scenario: Pipeline encerra naturalmente com lista vazia
  Given um workflow cujo lumn.test_source retorna {}
  When o desenvolvedor executa "lumn run" no workflow
  Then o exit code é 0
  And o JSON reporta items_in=0, items_out=0, status="ok"

Scenario: Validação detecta primitivo desconhecido no flow
  Given um workflow cujo flow contém um primitivo "merge" que não existe
  When o desenvolvedor executa "lumn validate" no workflow
  Then o exit code é 4 (ERR_UNKNOWN_PRIMITIVE)
  And stderr indica "merge" não é um primitivo válido com posição no flow

Scenario: Callable sem campo run é rejeitado
  Given um workflow cujo exec recebe uma table sem campo "run"
  When o desenvolvedor executa "lumn validate" no workflow
  Then o exit code é 5 (ERR_INVALID_SIGNATURE)
  And stderr indica que o callable precisa ter campo "run"

Scenario: Erro de runtime em callable é capturado
  Given um workflow cujo callable em exec lança error("falha intencional")
  When o desenvolvedor executa "lumn run" no workflow
  Then o exit code é 7 (ERR_RUNTIME)
  And o JSON contém error com type="runtime", message contendo "falha intencional" e posição do primitivo

Scenario: Workflow não encontrado
  Given que não existe pasta "inexistente/" no diretório atual
  When o desenvolvedor executa "lumn run inexistente"
  Then o exit code é 8 (ERR_WORKFLOW_NOT_FOUND)
```
