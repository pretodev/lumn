# Spec for Align Implementation with Revised Vision Document

branch: claude/feature/align-impl-with-vision-doc

## Summary

O Documento de Visão (`docs/index.md`) foi revisado com mudanças significativas na definição de workflows, na CLI e na estrutura de workspace. A implementação atual diverge dessa nova versão em vários pontos. Esta spec cobre as correções necessárias para realinhar o código com a especificação revisada — sem implementar features ainda não existentes no código (plugins, triggers, Web UI, etc.), apenas ajustando o que já existe para ficar compatível.

O escopo é cirúrgico: remover campos obsoletos, adotar o novo padrão de entrypoint (`lumn.lua`), atualizar a CLI para refletir os novos comandos, e garantir que a saída e os exemplos estejam consistentes com o documento.

## Functional Requirements

### 1. Remover `id` e `version` da definição de workflow

- O arquivo Lua do workflow **não deve mais conter** os campos `id` e `version` na table retornada.
- O engine deve parar de ler/exigir esses campos do retorno do arquivo Lua.
- A identidade (nome e tag de versão) é responsabilidade do runtime, definida em tempo de `lumn start` ou inferida do contexto (nome da pasta, `"latest"`).

### 2. Adotar `lumn.lua` como entrypoint padrão

- Quando nenhum argumento é fornecido, o runtime deve procurar `./lumn.lua` no diretório atual.
- Quando uma pasta é especificada:
  - Prioridade 1: `pasta/init.lua`
  - Prioridade 2: `pasta/lumn.lua`
- A flag `-f` deve ser suportada para forçar um arquivo ou pasta específico.
- Tabela de resolução:

| Situação                          | Entrypoint procurado | Prioridade |
| --------------------------------- | -------------------- | ---------- |
| Diretório atual, sem argumento    | `./lumn.lua`         | —          |
| Pasta especificada                | `pasta/init.lua`     | 1º         |
| Pasta especificada (sem init.lua) | `pasta/lumn.lua`     | 2º         |
| Arquivo especificado com `-f`     | o arquivo informado  | exato      |

### 3. Atualizar a CLI — remover comandos obsoletos

- Remover o comando `init` completamente.
- Remover o comando `exec` (se existir).
- Manter `run` e `validate` com as novas regras de resolução de entrypoint.

### 4. Atualizar a CLI — adicionar novos comandos

Todos os comandos de ciclo de vida do daemon devem ser adicionados:

- **`lumn start [name] [-f <alvo>]`** — Registra um workflow no daemon.
  - `name`: nome do workflow (padrão: nome da pasta atual).
  - Suporta `name:tag` (ex: `cancelamentos:1.2`). Sem tag, registra como `:latest`.
  - `-f` especifica arquivo ou pasta (padrão: `lumn.lua` do diretório atual).
  - Se daemon não estiver rodando, exibe erro claro.

- **`lumn stop <id|name>`** — Para a execução do workflow (desativa triggers). Erro se daemon não estiver rodando.

- **`lumn delete <id|name>`** — Remove o workflow do daemon permanentemente. Erro se daemon não estiver rodando.

- **`lumn restart <id|name>`** — Para e reinicia o workflow (aplica mudanças no arquivo). Erro se daemon não estiver rodando.

- **`lumn list`** — Tabela de todos os workflows no daemon com colunas: `ID · NAME · VERSION · STATUS · LAST RUN · FAILS · NEXT RUN`. Erro se daemon não estiver rodando.

- **`lumn watch [id|name]`** — TUI com DAG + logs em tempo real. Sem argumento, mostra todos. Erro se daemon não estiver rodando.

- **`lumn logs [id|name]`** — Stream de logs. Flags suportadas: `--lines <n>`, `--no-follow`, `--since <duration>`, `--level <level>`, `--step <nome>`. Erro se daemon não estiver rodando.

### 5. Atualizar `lumn run` e `lumn validate`

- `lumn run` sem argumento: procura `./lumn.lua` (modo dev, sem daemon, logs no terminal).
- `lumn run <id|name>`: tenta resolver no daemon primeiro (conexão obrigatória); se daemon não estiver rodando, prossegue para 2º pasta local, 3º arquivo `.lua`.
- `lumn run -f <arquivo|pasta>`: força execução de arquivo/pasta local (ignora daemon).
- `lumn validate` sem argumento: valida `./lumn.lua`.
- `lumn validate -f <arquivo|pasta>`: valida arquivo/pasta específico.

### 6. Geração de ID e tag de versionamento

- O ID do workflow é um hash curto gerado pelo daemon no momento do `lumn start`.
- O `VERSION` é a tag fornecida em `lumn start [name:tag]` — se não fornecida, default é `"latest"`.
- Na saída de `lumn run` (modo dev/standalone), o campo `workflow` usa o nome inferido (nome da pasta ou do arquivo) e `version` usa `"latest"`.

### 7. Saída JSON do `lumn run`

- A saída JSON de `lumn run` deve continuar com os campos `workflow`, `version`, `status`, `items_in`, `items_out`, `errors`, `duration_ms`.
- O campo `workflow` é preenchido com o nome inferido do contexto (nome da pasta atual, ou nome do arquivo sem extensão), não mais lido do arquivo Lua.
- O campo `version` é `"latest"` em modo dev/standalone.

### 8. Atualizar exemplos e fixtures de teste

- Todos os arquivos `.lua` de exemplo e teste que retornem `id` e `version` devem ser atualizados para remover esses campos.
- Arquivos `init.lua` de exemplo devem ser renomeados/adicionados como `lumn.lua` onde apropriado, seguindo as novas regras de entrypoint.

### 9. Atualizar README.md

- Remover referências a `lumn init`.
- Atualizar o quickstart para usar o novo fluxo (criar `lumn.lua` manualmente, `lumn run`, `lumn start`).
- Atualizar exemplos de workflow para não conter `id` e `version`.
- Atualizar a saída JSON de exemplo para refletir a nova geração de nome/versão.
- Atualizar a seção de estrutura de projeto e comandos CLI.

## Possible Edge Cases

- Workflow que especifica `id` ou `version` no retorno: o engine deve **ignorar** esses campos silenciosamente (não errar), mas não usá-los.
- Pasta que contém tanto `init.lua` quanto `lumn.lua`: `init.lua` tem prioridade, conforme a tabela de resolução.
- `lumn run` com argumento que não é uma instância no daemon nem um arquivo/pasta local: erro `ERR_WORKFLOW_NOT_FOUND`.
- Daemon não rodando para comandos que exigem daemon: exibir mensagem de erro clara (ex: `"daemon is not running — start it with 'lumn daemon start'"`).
- `lumn start` sem nome e sem estar dentro de uma pasta com `lumn.lua`: inferir nome do diretório atual.
- `lumn start` com `name:tag` onde `name` já existe com mesma tag: sobrescreve a instância existente (update in-place).

## Acceptance Criteria

- `lumn run` sem argumentos encontra e executa `./lumn.lua`.
- `lumn run -f <pasta>` resolve `init.lua` → `lumn.lua` dentro da pasta.
- `lumn validate` segue as mesmas regras de resolução de entrypoint.
- O comando `init` não existe mais na CLI.
- Os comandos `start`, `stop`, `delete`, `restart`, `list`, `watch` e `logs` estão registrados na CLI (mesmo que a integração com o daemon seja placeholder/stub).
- Arquivos Lua de workflow não contêm `id` nem `version` no retorno.
- O engine ignora `id` e `version` se presentes no retorno (backwards compat silenciosa).
- A saída JSON de `lumn run` preenche `workflow` pelo contexto (nome da pasta/arquivo) e `version` como `"latest"`.
- README.md reflete a nova CLI e os novos padrões de workflow.
- Todos os testes existentes passam após as mudanças.

## Open Questions

Todas as questões foram resolvidas:

- **`lumn start` com `name:tag` já existente:** sobrescreve (update in-place). Não exige `lumn delete` antes.
- **`lumn run <name>` — ordem de resolução:** tenta resolver no daemon primeiro. Se o daemon não estiver rodando, a tentativa de conexão falha e o comando prossegue para resolução local (pasta → arquivo `.lua`).
- **Campo `workflow` na saída JSON:** usa o nome do workflow (conforme fornecido em `lumn start` ou inferido do nome da pasta), não um slug derivado.

## Testing Guidelines

```gherkin
Scenario: Run with default lumn.lua entrypoint
  Given a directory containing a valid lumn.lua file
  When the user executes "lumn run" without arguments
  Then the workflow defined in lumn.lua is executed
  And the JSON output contains "workflow" set to the directory name
  And the JSON output contains "version" set to "latest"

Scenario: Run with folder resolution prefers init.lua over lumn.lua
  Given a folder "my_workflow" containing both init.lua and lumn.lua
  When the user executes "lumn run -f my_workflow"
  Then the workflow defined in init.lua is executed

Scenario: Run with folder falls back to lumn.lua
  Given a folder "my_workflow" containing only lumn.lua (no init.lua)
  When the user executes "lumn run -f my_workflow"
  Then the workflow defined in lumn.lua is executed

Scenario: Removed init command returns error
  When the user executes "lumn init pedidos"
  Then the CLI returns an error indicating the command does not exist

Scenario: Daemon commands error when daemon is not running
  When the user executes "lumn start cancelamentos"
  And the daemon is not running
  Then the CLI displays an error message indicating the daemon is not running

Scenario: Workflow file with legacy id and version fields
  Given a lumn.lua that returns a table containing "id" and "version" fields
  When the user executes "lumn run"
  Then the engine ignores the id and version fields from the file
  And the JSON output uses the inferred workflow name and "latest" as version

Scenario: Validate follows new entrypoint resolution
  Given a directory containing a valid lumn.lua file
  When the user executes "lumn validate" without arguments
  Then the lumn.lua file is validated successfully
```
