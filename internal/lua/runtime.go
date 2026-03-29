package lua

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/pretodev/lumn/pkg/errkind"
	golua "github.com/speedata/go-lua"
)

const (
	registryRuntimeKey      = "__lumn_runtime"
	registryStateKey        = "__lumn_state"
	registryPlaceholderMT   = "__lumn_placeholder_mt"
	registryRefPrefix       = "__lumn_ref_"
	kindField               = "__lumn_kind"
	execField               = "exec"
	onDataField             = "on_data"
	toField                 = "to"
	conditionField          = "condition"
	nameField               = "name"
	descriptionField        = "description"
	runField                = "run"
	missingSymbolField      = "__lumn_missing_symbol"
	unknownPrimitiveField   = "__lumn_unknown_primitive"
	sandboxErrorPrefix      = "__lumn_sandbox__:"
	runtimeErrorPrefix      = "__lumn_runtime__:"
	undefinedSymbolTemplate = "undefined symbol '%s'"
)

// Runtime owns the sandboxed Lua VM for a single workflow.
type Runtime struct {
	State        *golua.State
	WorkspaceDir string
	WorkflowDir  string
	SharedDir    string
	WorkflowName string
	stderr       io.Writer
	nextRef      uint64
}

func NewRuntime(workflowDir, workspaceDir string, stderr io.Writer) (*Runtime, error) {
	absDir, err := filepath.Abs(workflowDir)
	if err != nil {
		return nil, errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, fmt.Sprintf("resolve workflow directory: %v", err), err)
	}
	absWorkspaceDir, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, errkind.Wrap(errkind.ErrGeneric, errkind.TypeGeneric, fmt.Sprintf("resolve workspace directory: %v", err), err)
	}

	if stderr == nil {
		stderr = io.Discard
	}

	l := golua.NewState()
	rt := &Runtime{
		State:        l,
		WorkspaceDir: absWorkspaceDir,
		WorkflowDir:  absDir,
		SharedDir:    filepath.Join(absWorkspaceDir, "_shared"),
		WorkflowName: filepath.Base(absDir),
		stderr:       stderr,
	}

	l.PushUserData(rt)
	l.SetField(golua.RegistryIndex, registryRuntimeKey)

	for _, lib := range []struct {
		name string
		fn   golua.Function
	}{
		{name: "_G", fn: golua.BaseOpen},
		{name: "package", fn: golua.PackageOpen},
		{name: "coroutine", fn: golua.CoroutineOpen},
		{name: "string", fn: golua.StringOpen},
		{name: "table", fn: golua.TableOpen},
		{name: "math", fn: golua.MathOpen},
		{name: "bit32", fn: golua.Bit32Open},
		{name: "utf8", fn: golua.UTF8Open},
		{name: "os", fn: golua.OSOpen},
	} {
		golua.Require(l, lib.name, lib.fn, true)
		l.Pop(1)
	}

	rt.installPlaceholderMetatable()
	rt.installSandbox()
	rt.registerLumn()
	rt.installMissingGlobalHandler()

	return rt, nil
}

func (r *Runtime) Close() {}

func (r *Runtime) LoadWorkflow(initPath string) (string, error) {
	if err := golua.LoadFile(r.State, initPath, "t"); err != nil {
		defer r.State.SetTop(0)
		return "", r.normalizeLoadError(err)
	}
	if err := r.State.ProtectedCall(0, 1, 0); err != nil {
		defer r.State.SetTop(0)
		return "", r.normalizeLuaError(err)
	}
	if !r.State.IsTable(-1) {
		defer r.State.SetTop(0)
		return "", errkind.New(errkind.ErrStructure, errkind.TypeStructure, "workflow init.lua must return a table")
	}
	ref := r.StoreRef(-1)
	r.State.SetTop(0)
	return ref, nil
}

func (r *Runtime) StoreRef(index int) string {
	abs := r.State.AbsIndex(index)
	key := fmt.Sprintf("%s%d", registryRefPrefix, r.nextRef)
	r.nextRef++
	r.State.PushValue(abs)
	r.State.SetField(golua.RegistryIndex, key)
	return key
}

func (r *Runtime) DeleteRef(ref string) {
	r.State.PushNil()
	r.State.SetField(golua.RegistryIndex, ref)
}

func (r *Runtime) PushRef(ref string) {
	r.State.Field(golua.RegistryIndex, ref)
}

func (r *Runtime) SetExecutionState(stateRef string) {
	if stateRef == "" {
		r.State.PushNil()
		r.State.SetField(golua.RegistryIndex, registryStateKey)
		return
	}
	r.PushRef(stateRef)
	r.State.SetField(golua.RegistryIndex, registryStateKey)
}

func (r *Runtime) RefType(ref string) golua.Type {
	r.PushRef(ref)
	defer r.State.Pop(1)
	return r.State.TypeOf(-1)
}

func (r *Runtime) CallCallable(callableRef, inputRef, stateRef string) (string, error) {
	l := r.State
	top := l.Top()

	r.PushRef(callableRef)
	l.Field(-1, runField)
	if !l.IsFunction(-1) {
		l.SetTop(top)
		return "", errkind.New(errkind.ErrInvalidSignature, errkind.TypeInvalidSignature, "callable run must be a function")
	}
	l.Remove(-2)

	if inputRef == "" {
		l.PushNil()
	} else {
		r.PushRef(inputRef)
	}
	r.PushRef(stateRef)

	if err := l.ProtectedCall(2, 1, 0); err != nil {
		defer l.SetTop(top)
		return "", r.normalizeLuaError(err)
	}

	ref := r.StoreRef(-1)
	l.SetTop(top)
	return ref, nil
}

func (r *Runtime) CallFunction(fnRef string, resultCount int, argRefs ...string) ([]string, error) {
	l := r.State
	top := l.Top()

	r.PushRef(fnRef)
	for _, argRef := range argRefs {
		if argRef == "" {
			l.PushNil()
			continue
		}
		r.PushRef(argRef)
	}

	if err := l.ProtectedCall(len(argRefs), resultCount, 0); err != nil {
		defer l.SetTop(top)
		return nil, r.normalizeLuaError(err)
	}

	refs := make([]string, 0, resultCount)
	if resultCount > 0 {
		firstResult := l.Top() - resultCount + 1
		for i := 0; i < resultCount; i++ {
			refs = append(refs, r.StoreRef(firstResult+i))
		}
	}
	l.SetTop(top)
	return refs, nil
}

func (r *Runtime) TableLen(ref string) int {
	r.PushRef(ref)
	defer r.State.Pop(1)
	return r.State.RawLength(-1)
}

func (r *Runtime) ArrayValueRef(ref string, index int) string {
	r.PushRef(ref)
	defer r.State.Pop(1)
	r.State.RawGetInt(-1, index)
	valueRef := r.StoreRef(-1)
	r.State.Pop(1)
	return valueRef
}

func (r *Runtime) NewTableRef() string {
	r.State.NewTable()
	ref := r.StoreRef(-1)
	r.State.Pop(1)
	return ref
}

func (r *Runtime) CloneRef(ref string) (string, error) {
	r.PushRef(ref)
	value, err := r.toGoValue(-1, map[interface{}]bool{})
	r.State.Pop(1)
	if err != nil {
		return "", err
	}
	r.pushGoValue(value)
	cloned := r.StoreRef(-1)
	r.State.Pop(1)
	return cloned, nil
}

func (r *Runtime) IsMissingSymbolRef(ref string) bool {
	r.PushRef(ref)
	defer r.State.Pop(1)
	return r.hasBooleanField(-1, missingSymbolField)
}

func (r *Runtime) IsUnknownPrimitiveRef(ref string) bool {
	r.PushRef(ref)
	defer r.State.Pop(1)
	return r.hasBooleanField(-1, unknownPrimitiveField)
}

func (r *Runtime) TableStringFieldRef(ref, field string) (string, bool) {
	r.PushRef(ref)
	defer r.State.Pop(1)
	return r.tableStringField(-1, field)
}

func (r *Runtime) TableTypeFieldRef(ref, field string) golua.Type {
	r.PushRef(ref)
	defer r.State.Pop(1)
	r.State.Field(-1, field)
	defer r.State.Pop(1)
	return r.State.TypeOf(-1)
}

func (r *Runtime) TableRefFieldRef(ref, field string) (string, bool) {
	r.PushRef(ref)
	defer r.State.Pop(1)
	r.State.Field(-1, field)
	defer r.State.Pop(1)
	if r.State.IsNoneOrNil(-1) {
		return "", false
	}
	return r.StoreRef(-1), true
}

func (r *Runtime) TableBooleanFieldRef(ref, field string) bool {
	r.PushRef(ref)
	defer r.State.Pop(1)
	return r.hasBooleanField(-1, field)
}

func (r *Runtime) TableKindRef(ref string) (string, bool) {
	return r.TableStringFieldRef(ref, kindField)
}

func (r *Runtime) normalizeLoadError(err error) error {
	msg := r.stackMessage(err)
	switch {
	case errors.Is(err, golua.SyntaxError):
		return errkind.Wrap(errkind.ErrSyntax, errkind.TypeSyntax, msg, err)
	default:
		return r.normalizeLuaError(err)
	}
}

func (r *Runtime) normalizeLuaError(err error) error {
	msg := r.stackMessage(err)
	code := errkind.ErrRuntime
	typ := errkind.TypeRuntime

	switch {
	case strings.HasPrefix(msg, sandboxErrorPrefix):
		msg = strings.TrimPrefix(msg, sandboxErrorPrefix)
		code = errkind.ErrSandbox
		typ = errkind.TypeSandbox
	case strings.HasPrefix(msg, runtimeErrorPrefix):
		msg = strings.TrimPrefix(msg, runtimeErrorPrefix)
	}

	return errkind.Wrap(code, typ, msg, err)
}

func (r *Runtime) stackMessage(fallback error) string {
	if msg, ok := r.State.ToString(-1); ok && msg != "" {
		return msg
	}
	if fallback != nil {
		return fallback.Error()
	}
	return "lua error"
}

func runtimeFromState(l *golua.State) *Runtime {
	l.Field(golua.RegistryIndex, registryRuntimeKey)
	defer l.Pop(1)
	if rt, ok := l.ToUserData(-1).(*Runtime); ok {
		return rt
	}
	return nil
}

func pushExecutionState(l *golua.State) bool {
	l.Field(golua.RegistryIndex, registryStateKey)
	return l.IsTable(-1)
}

func (r *Runtime) installSandbox() {
	l := r.State

	l.PushGoFunction(printFunc)
	l.SetGlobal("print")

	for _, name := range []string{"dofile", "loadfile", "load"} {
		r.pushBlockedClosure(name)
		l.SetGlobal(name)
	}

	r.installRequire()
	r.installPackageGuards()
	r.installBlockedLibrary("io")
	r.installBlockedLibrary("debug")
	r.installOSTable()
}

func (r *Runtime) installRequire() {
	r.State.PushGoFunction(requireFunc)
	r.State.SetGlobal("require")
}

func (r *Runtime) installPackageGuards() {
	l := r.State

	l.Global("package")
	if !l.IsTable(-1) {
		l.Pop(1)
		return
	}

	l.NewTable()
	l.SetField(-2, "searchers")
	l.PushString("")
	l.SetField(-2, "path")
	l.PushString("")
	l.SetField(-2, "cpath")
	r.pushBlockedClosure("package.loadlib")
	l.SetField(-2, "loadlib")
	l.Pop(1)
}

func (r *Runtime) installBlockedLibrary(name string) {
	l := r.State

	l.NewTable()
	l.NewTable()
	l.PushGoFunction(blockedLibraryIndex)
	l.SetField(-2, "__index")
	l.PushGoFunction(blockedLibraryNewIndex)
	l.SetField(-2, "__newindex")
	l.SetMetaTable(-2)
	l.SetGlobal(name)
}

func (r *Runtime) installOSTable() {
	l := r.State

	l.Global("os")
	if !l.IsTable(-1) {
		l.Pop(1)
		return
	}

	originalOS := l.AbsIndex(-1)
	l.NewTable()
	newOS := l.AbsIndex(-1)

	for _, field := range []string{"clock", "time", "date", "difftime"} {
		l.Field(originalOS, field)
		l.SetField(newOS, field)
	}

	for _, blocked := range []string{"execute", "exit", "remove", "rename", "setlocale", "tmpname", "getenv"} {
		r.pushBlockedClosure("os." + blocked)
		l.SetField(newOS, blocked)
	}

	l.NewTable()
	l.PushGoFunction(osIndex)
	l.SetField(-2, "__index")
	l.PushGoFunction(osNewIndex)
	l.SetField(-2, "__newindex")
	l.SetMetaTable(newOS)

	l.Remove(originalOS)
	l.SetGlobal("os")
}

func (r *Runtime) registerLumn() {
	l := r.State

	l.NewTable()
	l.PushGoFunction(testSource)
	l.SetField(-2, "test_source")
	l.PushGoFunction(lumnGet)
	l.SetField(-2, "get")
	l.PushGoFunction(lumnSet)
	l.SetField(-2, "set")

	l.SetGlobal("lumn")

	for name, fn := range map[string]golua.Function{
		"call":   primitiveCall,
		"set":    primitiveSet,
		"filter": primitiveFilter,
		"tap":    primitiveTap,
	} {
		l.PushGoFunction(fn)
		l.SetGlobal(name)
	}
}

func (r *Runtime) installMissingGlobalHandler() {
	l := r.State

	l.PushGlobalTable()
	l.NewTable()
	l.PushGoFunction(missingGlobal)
	l.SetField(-2, "__index")
	l.SetMetaTable(-2)
	l.Pop(1)
}

func (r *Runtime) installPlaceholderMetatable() {
	l := r.State

	l.NewTable()
	l.PushGoFunction(missingCall)
	l.SetField(-2, "__call")
	for _, metamethod := range []string{
		"__index",
		"__newindex",
		"__add",
		"__sub",
		"__mul",
		"__div",
		"__mod",
		"__pow",
		"__concat",
		"__len",
		"__unm",
		"__pairs",
		"__ipairs",
	} {
		l.PushGoFunction(missingUsage)
		l.SetField(-2, metamethod)
	}
	l.SetField(golua.RegistryIndex, registryPlaceholderMT)
}

func (r *Runtime) pushBlockedClosure(name string) {
	r.State.PushString(name)
	r.State.PushGoClosure(blockedFunc, 1)
}

func pushPrefixedError(l *golua.State, prefix, message string) int {
	l.PushString(prefix + message)
	l.Error()
	return 0
}

func blockedFunc(l *golua.State) int {
	name, _ := l.ToString(golua.UpValueIndex(1))
	return pushPrefixedError(l, sandboxErrorPrefix, fmt.Sprintf("access to %q is blocked", name))
}

func blockedLibraryIndex(l *golua.State) int {
	l.CheckStack(1)
	field := golua.CheckString(l, 2)
	l.PushString("library." + field)
	l.PushGoClosure(blockedFunc, 1)
	return 1
}

func blockedLibraryNewIndex(l *golua.State) int {
	field := golua.CheckString(l, 2)
	return pushPrefixedError(l, sandboxErrorPrefix, fmt.Sprintf("access to %q is blocked", "library."+field))
}

func osIndex(l *golua.State) int {
	field := golua.CheckString(l, 2)
	return pushPrefixedError(l, sandboxErrorPrefix, fmt.Sprintf("access to %q is blocked", "os."+field))
}

func osNewIndex(l *golua.State) int {
	field := golua.CheckString(l, 2)
	return pushPrefixedError(l, sandboxErrorPrefix, fmt.Sprintf("access to %q is blocked", "os."+field))
}

func missingGlobal(l *golua.State) int {
	rt := runtimeFromState(l)
	name := golua.CheckString(l, 2)
	rt.pushMissingSymbol(name)
	return 1
}

func missingCall(l *golua.State) int {
	rt := runtimeFromState(l)
	name, _ := rt.tableStringField(1, nameField)
	rt.pushUnknownPrimitive(name)
	return 1
}

func missingUsage(l *golua.State) int {
	rt := runtimeFromState(l)
	name, _ := rt.tableStringField(1, nameField)
	return pushPrefixedError(l, runtimeErrorPrefix, fmt.Sprintf(undefinedSymbolTemplate, name))
}

func printFunc(l *golua.State) int {
	rt := runtimeFromState(l)
	originalTop := l.Top()
	parts := make([]string, 0, originalTop)
	for i := 1; i <= originalTop; i++ {
		beforeTop := l.Top()
		s, ok := golua.ToStringMeta(l, i)
		afterTop := l.Top()
		if afterTop > beforeTop {
			l.Pop(afterTop - beforeTop)
		}
		if !ok {
			s = fmt.Sprintf("<%s>", l.TypeOf(i))
		}
		parts = append(parts, s)
	}
	fmt.Fprintln(rt.stderr, strings.Join(parts, "\t"))
	return 0
}

func requireFunc(l *golua.State) int {
	rt := runtimeFromState(l)
	moduleName := golua.CheckString(l, 1)

	l.Field(golua.RegistryIndex, "_LOADED")
	l.Field(-1, moduleName)
	if !l.IsNil(-1) {
		l.Remove(-2)
		return 1
	}
	l.Pop(1)

	modulePath, err := rt.resolveModulePath(moduleName)
	if err != nil {
		return pushPrefixedError(l, sandboxErrorPrefix, err.Error())
	}

	if err := golua.LoadFile(l, modulePath, "t"); err != nil {
		return pushPrefixedError(l, runtimeErrorPrefix, rt.normalizeLoadError(err).Error())
	}
	if err := l.ProtectedCall(0, 1, 0); err != nil {
		return pushPrefixedError(l, runtimeErrorPrefix, rt.normalizeLuaError(err).Error())
	}
	if l.IsNil(-1) {
		l.Pop(1)
		l.PushBoolean(true)
	}
	l.PushValue(-1)
	l.SetField(-3, moduleName)
	l.Remove(-2)
	return 1
}

func lumnGet(l *golua.State) int {
	key := golua.CheckString(l, 1)
	if !pushExecutionState(l) {
		return pushPrefixedError(l, runtimeErrorPrefix, "lumn.get is only available during execution")
	}
	l.Field(-1, key)
	l.Remove(-2)
	return 1
}

func lumnSet(l *golua.State) int {
	key := golua.CheckString(l, 1)
	if !pushExecutionState(l) {
		return pushPrefixedError(l, runtimeErrorPrefix, "lumn.set is only available during execution")
	}
	if l.IsNoneOrNil(2) {
		l.PushNil()
	} else {
		l.PushValue(2)
	}
	l.SetField(-2, key)
	l.Pop(1)
	return 0
}

func primitiveCall(l *golua.State) int {
	return primitiveNode(l, "call")
}

func primitiveSet(l *golua.State) int {
	return primitiveNode(l, "set")
}

func primitiveFilter(l *golua.State) int {
	return primitiveNode(l, "filter")
}

func primitiveTap(l *golua.State) int {
	return primitiveNode(l, "tap")
}

func primitiveNode(l *golua.State, kind string) int {
	if l.IsTable(1) {
		l.PushValue(1)
	} else {
		l.NewTable()
	}
	l.PushString(kind)
	l.SetField(-2, kindField)
	return 1
}

func testSource(l *golua.State) int {
	if !l.IsTable(1) {
		golua.ArgumentError(l, 1, "table expected")
	}

	l.NewTable()
	l.PushString("lumn.test_source")
	l.SetField(-2, nameField)
	l.PushString("builtin source for tests and local development")
	l.SetField(-2, descriptionField)
	l.PushValue(1)
	l.PushGoClosure(testSourceRun, 1)
	l.SetField(-2, runField)
	return 1
}

func testSourceRun(l *golua.State) int {
	l.PushValue(golua.UpValueIndex(1))
	return 1
}

func (r *Runtime) pushMissingSymbol(name string) {
	l := r.State
	l.NewTable()
	l.PushString(name)
	l.SetField(-2, nameField)
	l.PushBoolean(true)
	l.SetField(-2, missingSymbolField)
	l.Field(golua.RegistryIndex, registryPlaceholderMT)
	l.SetMetaTable(-2)
}

func (r *Runtime) pushUnknownPrimitive(name string) {
	l := r.State
	l.NewTable()
	l.PushString(name)
	l.SetField(-2, nameField)
	l.PushBoolean(true)
	l.SetField(-2, unknownPrimitiveField)
}

func (r *Runtime) hasBooleanField(index int, field string) bool {
	abs := r.State.AbsIndex(index)
	r.State.Field(abs, field)
	defer r.State.Pop(1)
	return r.State.ToBoolean(-1)
}

func (r *Runtime) tableStringField(index int, field string) (string, bool) {
	abs := r.State.AbsIndex(index)
	r.State.Field(abs, field)
	defer r.State.Pop(1)
	return r.State.ToString(-1)
}

func (r *Runtime) resolveModulePath(module string) (string, error) {
	if module == "" {
		return "", fmt.Errorf("module name cannot be empty")
	}
	if filepath.IsAbs(module) {
		return "", fmt.Errorf("module path %q is outside the workflow sandbox", module)
	}
	if strings.Contains(module, "..") {
		return "", fmt.Errorf("module path %q is outside the workflow sandbox", module)
	}

	modulePath := strings.ReplaceAll(module, ".", string(filepath.Separator))
	modulePath = strings.ReplaceAll(modulePath, "/", string(filepath.Separator))
	modulePath = filepath.Clean(modulePath + ".lua")

	roots := []string{r.WorkflowDir, r.SharedDir}
	for _, root := range roots {
		if root == "" {
			continue
		}
		candidate := filepath.Join(root, modulePath)
		absCandidate, err := filepath.Abs(candidate)
		if err != nil {
			return "", err
		}
		if !isWithinRoot(absCandidate, root) {
			continue
		}
		if _, err := os.Stat(absCandidate); err == nil {
			return absCandidate, nil
		}
	}

	return "", fmt.Errorf("module %q could not be resolved inside %s", module, r.WorkflowName)
}

func isWithinRoot(candidate, root string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
}

func (r *Runtime) toGoValue(index int, seen map[interface{}]bool) (any, error) {
	l := r.State
	switch l.TypeOf(index) {
	case golua.TypeNil:
		return nil, nil
	case golua.TypeBoolean:
		return l.ToBoolean(index), nil
	case golua.TypeString:
		s, _ := l.ToString(index)
		return s, nil
	case golua.TypeNumber:
		if l.IsInteger(index) {
			i, _ := l.ToInteger64(index)
			return i, nil
		}
		n, _ := l.ToNumber(index)
		return n, nil
	case golua.TypeTable:
		identity := l.ToValue(index)
		if seen[identity] {
			return nil, errkind.New(errkind.ErrRuntime, errkind.TypeRuntime, "cyclic table values are not supported")
		}
		seen[identity] = true
		defer delete(seen, identity)

		abs := l.AbsIndex(index)
		values := map[any]any{}
		l.PushNil()
		for l.Next(abs) {
			key, err := r.toGoValue(-2, seen)
			if err != nil {
				l.Pop(2)
				return nil, err
			}
			value, err := r.toGoValue(-1, seen)
			if err != nil {
				l.Pop(2)
				return nil, err
			}
			values[key] = value
			l.Pop(1)
		}
		return values, nil
	default:
		return nil, errkind.New(errkind.ErrRuntime, errkind.TypeRuntime, fmt.Sprintf("unsupported Lua value type %s", l.TypeOf(index)))
	}
}

func (r *Runtime) pushGoValue(value any) {
	switch v := value.(type) {
	case nil:
		r.State.PushNil()
	case bool:
		r.State.PushBoolean(v)
	case string:
		r.State.PushString(v)
	case int:
		r.State.PushInteger(v)
	case int64:
		r.State.PushInteger64(v)
	case float64:
		if math.Trunc(v) == v {
			r.State.PushInteger64(int64(v))
		} else {
			r.State.PushNumber(v)
		}
	case []any:
		r.State.CreateTable(len(v), 0)
		for i, item := range v {
			r.pushGoValue(item)
			r.State.RawSetInt(-2, i+1)
		}
	case map[any]any:
		r.State.NewTable()
		for key, item := range v {
			r.pushGoValue(key)
			r.pushGoValue(item)
			r.State.RawSet(-3)
		}
	default:
		r.State.PushNil()
	}
}
