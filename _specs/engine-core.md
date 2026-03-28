# Spec for Engine Core

branch: claude/feature/engine-core

## Summary

Implementar o núcleo do engine lumn: uma VM Lua embarcada com sandbox de segurança, o global `lumn` expondo os primitivos fundamentais da DSL (`exec`, `set`, `filter`, `tap`), um DAG builder que monta o pipeline a partir da table `flow`, um executor sequencial que processa itens através do pipeline, e uma CLI básica com os comandos `init`, `run` e `validate`.

Esta é a fundação sobre a qual todo o restante da plataforma será construído. Ao final desta entrega, um desenvolvedor deve conseguir criar um projeto, escrever um workflow com funções Lua puras como callables, validar a estrutura e executar o pipeline localmente.

## Functional Requirements

### VM Lua Embarcada

- Embarcar uma VM Lua no binário Go (ex: via gopher-lua ou similar)
- Aplicar sandbox desde a primeira iteração: remover acesso a `os`, `io`, `loadfile`, `dofile`, `debug` e qualquer função que permita I/O ou execução arbitrária
- Manter `require` funcional apenas para módulos locais do projeto (resolução relativa ao diretório do workflow)
- Injetar o global `lumn` no ambiente Lua antes da execução do workflow

### Global `lumn` com Primitivos da DSL

- Expor os quatro primitivos fundamentais como funções globais acessíveis no contexto do `flow`:
  - **`exec(callable)`** — executa um callable (função Lua pura nesta fase); o retorno fica disponível como `res` no próximo `set`
  - **`set(fn(res, item, ctx) -> item)`** — transforma o item atual; recebe o resultado do `exec` anterior (ou `nil`), o item e o contexto global
  - **`filter(fn(item, ctx) -> bool)`** — remove itens onde a função retorna `false`; item não é mutado
  - **`tap(fn(item, ctx))`** — executa efeito colateral puro; retorno é descartado, item nunca é mutado
- Quando a lista de itens fica vazia em qualquer ponto do pipeline, o workflow encerra naturalmente sem erro

### DAG Builder

- Parsear a table retornada pelo `init.lua` do workflow
- Validar a presença dos campos obrigatórios: `id` (string), `version` (string), `flow` (table)
- Construir uma representação interna do pipeline a partir da sequência de primitivos em `flow`
- Rejeitar primitivos desconhecidos com erro claro indicando nome e posição

### Executor Sequencial

- Processar itens um a um, na ordem em que aparecem na lista
- Manter o contrato de `res`: o retorno de `exec` é passado ao próximo `set`; se não houver `exec` anterior, `res` é `nil`
- Manter `ctx` compartilhado entre todos os itens de uma execução
- O primeiro `exec` do pipeline é o responsável por produzir a lista inicial de itens
- Reportar resultado da execução no stdout: quantos itens entraram, quantos saíram, erros encontrados

### CLI Básica

- **`lumn init <nome>`** — cria o esqueleto mínimo: uma pasta `<nome>/` com um arquivo `init.lua` contendo a estrutura base de um workflow (table com `id`, `version`, `flow` vazios)
- **`lumn run <workflow>`** — carrega o `init.lua` do workflow indicado, monta o pipeline via DAG builder e executa com o executor sequencial
- **`lumn validate <workflow>`** — validação completa: verifica sintaxe Lua, parseia a table retornada, valida campos obrigatórios (`id`, `version`, `flow`), verifica que todos os primitivos no `flow` são conhecidos e que as assinaturas estão corretas (ex: `set` recebe função, `filter` recebe função)

## Possible Edge Cases

- Workflow cujo `init.lua` não retorna uma table (ex: retorna `nil` ou string)
- Primitivo `set` que não retorna o item (retorno `nil`) — deve gerar erro claro
- Função passada a `exec` que lança erro Lua — deve capturar e reportar com contexto (nome do workflow, posição no pipeline)
- Workflow com `flow` vazio (table sem primitivos) — deve executar sem erro e reportar 0 itens processados
- `require` tentando acessar path fora do diretório do projeto — deve ser bloqueado pelo sandbox
- `init.lua` que tenta acessar `os.execute` ou `io.open` — deve falhar com erro de sandbox antes da execução
- `lumn run` em diretório que não contém workflow válido — mensagem de erro orientando o usuário
- `lumn validate` em arquivo com sintaxe Lua inválida — reportar linha e coluna do erro de parse

## Acceptance Criteria

- Um workflow simples com `exec` → `set` → `filter` → `tap` executa corretamente via `lumn run` e produz output esperado no stdout
- `lumn init meu-workflow` cria `meu-workflow/init.lua` com template válido
- `lumn validate` rejeita workflows com campos obrigatórios ausentes, primitivos desconhecidos ou assinaturas inválidas, com mensagens de erro claras
- A VM Lua bloqueia acesso a `os`, `io`, `loadfile`, `dofile` e `debug` — tentativas resultam em erro de sandbox
- `require` funciona para módulos locais do projeto e falha para paths externos
- Pipeline com lista vazia de itens encerra sem erro
- Erros em runtime Lua (dentro de callables) são capturados e reportados com contexto suficiente para debug
- O binário `lumn` é compilável como single binary Go sem dependências externas

## Open Questions

- Qual biblioteca Go para a VM Lua? `gopher-lua` é a mais madura, mas `golua` e outras alternativas existem. Decisão impacta performance e compatibilidade com Lua 5.1 vs 5.4.
- O `ctx` deve ser pré-populável via CLI flags (ex: `lumn run order_cancel --ctx.token=abc`)? Útil para testes, mas adiciona complexidade ao parser de argumentos.
- Como o primeiro `exec` produz a lista inicial de itens? Nesta fase com funções puras, a convenção é que o callable retorne uma table-array? Ou existe um primitivo `source` implícito?
- O `lumn validate` deve ter exit codes específicos para cada tipo de erro (sintaxe, estrutura, sandbox) ou basta exit code 1 genérico?
- Formato de output do `lumn run`: texto humano simples ou JSON estruturado (ou flag para escolher)?

## Testing Guidelines

```gherkin
Scenario: Executar workflow simples com pipeline completo
  Given um workflow "pedidos" com exec que retorna 3 itens, set que adiciona campo, filter que remove 1 item e tap que loga
  When o desenvolvedor executa "lumn run pedidos"
  Then o stdout reporta 3 itens de entrada e 2 itens processados com sucesso

Scenario: Inicializar projeto com esqueleto mínimo
  Given um diretório vazio
  When o desenvolvedor executa "lumn init meu-workflow"
  Then a pasta "meu-workflow/" é criada contendo "init.lua" com template válido
  And "lumn validate meu-workflow" passa sem erros

Scenario: Validação rejeita workflow com campo obrigatório ausente
  Given um workflow cujo init.lua retorna table sem campo "id"
  When o desenvolvedor executa "lumn validate" no workflow
  Then o comando falha com mensagem indicando que "id" é obrigatório
  And o exit code é diferente de zero

Scenario: Sandbox bloqueia acesso a funções perigosas
  Given um workflow cujo init.lua contém chamada a "os.execute('rm -rf /')"
  When o desenvolvedor executa "lumn run" no workflow
  Then a execução falha com erro de sandbox antes de qualquer side effect
  And a mensagem indica que "os" não está disponível no ambiente

Scenario: Pipeline encerra naturalmente com lista vazia
  Given um workflow cujo primeiro exec retorna uma lista vazia
  When o desenvolvedor executa "lumn run" no workflow
  Then a execução completa sem erro
  And o stdout reporta 0 itens processados

Scenario: Validação detecta primitivo desconhecido no flow
  Given um workflow cujo flow contém um primitivo "merge" que não existe
  When o desenvolvedor executa "lumn validate" no workflow
  Then o comando falha com mensagem indicando que "merge" não é um primitivo válido
  And a posição do primitivo no flow é incluída na mensagem
```
