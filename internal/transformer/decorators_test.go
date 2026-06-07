package transformer_test

import (
	"testing"
)

// All expected text in this file is byte-for-byte what rbxtsc 3.0.0 emits for
// the same source (compiled through testdata/diff/project with
// "experimentalDecorators": true on 2026-06-07; header and trailing return
// stripped — those belong to TransformSourceFile, not the statement list
// under test).

// TestDecorators pins the full legacy-decorator interleave (digest §2.9/§5.4):
//
//   - evaluation order: INSTANCE members (declaration order), then STATIC
//     members (declaration order), then CLASS decorators — emitted LAST inside
//     the class do-block;
//   - per target, decorator expressions initialize first-to-last but apply
//     LAST-TO-FIRST (TC39 bottom-up; `@Component @ComponentFactory("hi")`
//     applies ComponentFactory("hi") first);
//   - method decorators: `local _descriptor = deco(Class, KEY, { value =
//     Class[KEY] })` + `if _descriptor then Class[KEY] = _descriptor.value end`;
//   - property decorators: `deco(Class, KEY)` call statement;
//   - parameter decorators: `deco(Class, KEY, i)` with the 0-BASED index, KEY
//     nil for constructor parameters; they sandwich between their member's
//     initializers and finalizers, and the LAST parameter applies first;
//   - shouldInline: a mutating expression spills to `local _decorator = ...`
//     when it is not the last decorator (`@mutableDeco @Component`) or when
//     parameter decorators must run in between (`@ComponentFactory("hi")` with
//     a decorated constructor parameter);
//   - computed method names reuse the `_key` temp pinned by
//     transformMethodDeclaration (decorator key-pinning).
func TestDecorators(t *testing.T) {
	got, s := renderClassFixture(t, "src/decorators.ts")

	want := `local function Component(ctor)
	print("Component", ctor)
end
local function ComponentFactory(arg)
	return function(ctor)
		print("ComponentFactory", arg, ctor)
	end
end
local function LogProp(target, key)
	print("LogProp", target, key)
end
local function LogMethod(target, key, descriptor)
	print("LogMethod", target, key, descriptor)
end
local function LogParam(target, key, index)
	print("LogParam", target, key, index)
end
local mutableDeco = function(ctor)
	print("mutable", ctor)
end
local Decorated
do
	Decorated = setmetatable({}, {
		__tostring = function()
			return "Decorated"
		end,
	})
	Decorated.__index = Decorated
	function Decorated.new(...)
		local self = setmetatable({}, Decorated)
		return self:constructor(...) or self
	end
	function Decorated:constructor(x)
		self.value = 1
		print(x)
	end
	function Decorated:method(a, b)
		print(a, b)
	end
	function Decorated:smethod()
	end
	Decorated.svalue = 2
	LogProp(Decorated, "value")
	LogParam(Decorated, "method", 1)
	LogParam(Decorated, "method", 0)
	local _descriptor = LogMethod(Decorated, "method", {
		value = Decorated.method,
	})
	if _descriptor then
		Decorated.method = _descriptor.value
	end
	LogProp(Decorated, "svalue")
	local _descriptor_1 = LogMethod(Decorated, "smethod", {
		value = Decorated.smethod,
	})
	if _descriptor_1 then
		Decorated.smethod = _descriptor_1.value
	end
	local _decorator = ComponentFactory("hi")
	LogParam(Decorated, nil, 0)
	Decorated = _decorator(Decorated) or Decorated
	Decorated = Component(Decorated) or Decorated
end
local MutableDecorated
do
	MutableDecorated = setmetatable({}, {
		__tostring = function()
			return "MutableDecorated"
		end,
	})
	MutableDecorated.__index = MutableDecorated
	function MutableDecorated.new(...)
		local self = setmetatable({}, MutableDecorated)
		return self:constructor(...) or self
	end
	function MutableDecorated:constructor()
	end
	local _decorator = mutableDeco
	MutableDecorated = Component(MutableDecorated) or MutableDecorated
	MutableDecorated = _decorator(MutableDecorated) or MutableDecorated
end
local KEY = "dyn"
local ComputedDecorated
do
	ComputedDecorated = setmetatable({}, {
		__tostring = function()
			return "ComputedDecorated"
		end,
	})
	ComputedDecorated.__index = ComputedDecorated
	function ComputedDecorated.new(...)
		local self = setmetatable({}, ComputedDecorated)
		return self:constructor(...) or self
	end
	function ComputedDecorated:constructor()
	end
	local _key = KEY
	ComputedDecorated[_key] = function(self) end
	local _descriptor = LogMethod(ComputedDecorated, _key, {
		value = ComputedDecorated[_key],
	})
	if _descriptor then
		ComputedDecorated[_key] = _descriptor.value
	end
end
local Exported
do
	Exported = setmetatable({}, {
		__tostring = function()
			return "Exported"
		end,
	})
	Exported.__index = Exported
	function Exported.new(...)
		local self = setmetatable({}, Exported)
		return self:constructor(...) or self
	end
	function Exported:constructor()
	end
	function Exported:multi()
	end
	local _descriptor = LogMethod(Exported, "multi", {
		value = Exported.multi,
	})
	if _descriptor then
		Exported.multi = _descriptor.value
	end
	local _descriptor_1 = LogMethod(Exported, "multi", {
		value = Exported.multi,
	})
	if _descriptor_1 then
		Exported.multi = _descriptor_1.value
	end
	Exported = Component(Exported) or Exported
end
print(Decorated, MutableDecorated, ComputedDecorated, Exported, mutableDeco)
`
	if got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if ds := s.Diags.Flush(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}
}

// TestDecoratorErrorBoundary pins the Phase 3c Task 3 acceptance shape — a
// `@ReactComponent export class ErrorBoundary extends React.Component<P, S>`
// mirroring randomness client/ui/error-boundary.tsx (no JSX syntax):
//
//   - the class decorator emits after all members:
//     `ErrorBoundary = ReactComponent(ErrorBoundary) or ErrorBoundary`;
//   - a decorator FACTORY on a property/method inlines as a call-of-call
//     (`LogF("state")(ErrorBoundary, "currentState")`) — the factory call is
//     the last decorator on its target so shouldInline allows it despite
//     mutating;
//   - a property-access decorator (`@ns.deco`) also inlines:
//     `ns.deco(ErrorBoundary, "other")`.
func TestDecoratorErrorBoundary(t *testing.T) {
	got, s := renderClassFixture(t, "src/errorboundary.ts")

	want := `local function ReactComponent(ctor)
	print("ReactComponent", ctor)
end
local function LogF(tag)
	return function(target, key)
		print("LogF", tag, target, key)
	end
end
local function LogMF(tag)
	return function(target, key, descriptor)
		print("LogMF", tag, target, key, descriptor)
	end
end
local ns = {
	deco = function(target, key)
		print("ns.deco", target, key)
	end,
}
local Component
do
	Component = setmetatable({}, {
		__tostring = function()
			return "Component"
		end,
	})
	Component.__index = Component
	function Component.new(...)
		local self = setmetatable({}, Component)
		return self:constructor(...) or self
	end
	function Component:constructor()
	end
	function Component:setState(s)
		self.state = s
	end
end
local React = {
	Component = Component,
}
local ErrorBoundary
do
	local super = React.Component
	ErrorBoundary = setmetatable({}, {
		__tostring = function()
			return "ErrorBoundary"
		end,
		__index = super,
	})
	ErrorBoundary.__index = ErrorBoundary
	function ErrorBoundary.new(...)
		local self = setmetatable({}, ErrorBoundary)
		return self:constructor(...) or self
	end
	function ErrorBoundary:constructor(...)
		super.constructor(self, ...)
		self.currentState = {
			hasError = false,
		}
		self.other = 1
	end
	function ErrorBoundary:componentDidCatch(message)
		self:setState({
			hasError = true,
		})
		print(message)
	end
	LogF("state")(ErrorBoundary, "currentState")
	ns.deco(ErrorBoundary, "other")
	local _descriptor = LogMF("derived")(ErrorBoundary, "componentDidCatch", {
		value = ErrorBoundary.componentDidCatch,
	})
	if _descriptor then
		ErrorBoundary.componentDidCatch = _descriptor.value
	end
	ErrorBoundary = ReactComponent(ErrorBoundary) or ErrorBoundary
end
print(ErrorBoundary)
`
	if got != want {
		t.Errorf("rendered output differs from rbxtsc:\ngot:\n%s\nwant:\n%s", got, want)
	}
	if ds := s.Diags.Flush(); len(ds) != 0 {
		t.Errorf("unexpected diagnostics: %v", ds)
	}
}
