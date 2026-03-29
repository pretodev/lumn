package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/pretodev/lumn/internal/engine"
	"github.com/pretodev/lumn/pkg/errkind"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: lumn <init|validate|run> <workflow>")
		return int(errkind.ErrGeneric)
	}

	switch args[0] {
	case "init":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn init <name>")
			return int(errkind.ErrGeneric)
		}
		if err := initWorkflow(args[1]); err != nil {
			fmt.Fprintln(stderr, errkind.Format(err))
			return errkind.ExitStatus(err)
		}
		fmt.Fprintf(stdout, "created %s\n", filepath.Join(args[1], "init.lua"))
		return int(errkind.OK)
	case "validate":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn validate <workflow|init.lua>")
			return int(errkind.ErrGeneric)
		}
		if err := engine.ValidateTarget(args[1], stderr); err != nil {
			fmt.Fprintln(stderr, errkind.Format(err))
			return errkind.ExitStatus(err)
		}
		return int(errkind.OK)
	case "run":
		if len(args) != 2 {
			fmt.Fprintln(stderr, "usage: lumn run <workflow|init.lua>")
			return int(errkind.ErrGeneric)
		}
		report, code := engine.RunTarget(args[1], stderr)
		enc := json.NewEncoder(stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(report)
		return code
	default:
		fmt.Fprintln(stderr, "usage: lumn <init|validate|run> <workflow>")
		return int(errkind.ErrGeneric)
	}
}

type scaffoldData struct {
	ID string
}

func initWorkflow(name string) error {
	if name == "" {
		return errkind.New(errkind.ErrGeneric, errkind.TypeGeneric, "workflow name is required")
	}

	if _, err := os.Stat(name); err == nil {
		return errkind.New(errkind.ErrGeneric, errkind.TypeGeneric, "target already exists")
	}

	if err := os.MkdirAll(name, 0o755); err != nil {
		return errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, err.Error(), err)
	}

	initPath := filepath.Join(name, "init.lua")
	file, err := os.Create(initPath)
	if err != nil {
		return errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, err.Error(), err)
	}
	defer file.Close()

	tpl := template.Must(template.New("init").Parse(initTemplate))
	if err := tpl.Execute(file, scaffoldData{ID: filepath.Base(name)}); err != nil {
		return errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, err.Error(), err)
	}

	return nil
}

const initTemplate = `local items = {
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
  id = "{{ .ID }}",
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
`
