function Component(ctor: defined) {
	print("Component", ctor);
}

function ComponentFactory(arg: string) {
	return (ctor: defined) => {
		print("ComponentFactory", arg, ctor);
	};
}

function LogProp(target: defined, key: string) {
	print("LogProp", target, key);
}

function LogPropFactory(tag: string) {
	return (target: defined, key: string) => {
		print("LogPropFactory", tag, target, key);
	};
}

function LogMethod(target: defined, key: string, descriptor: { value: unknown }) {
	print("LogMethod", target, key, descriptor);
}

function LogMethodFactory(tag: string) {
	return (target: defined, key: string, descriptor: { value: unknown }) => {
		print("LogMethodFactory", tag, target, key, descriptor);
	};
}

function LogParam(target: defined, key: string | undefined, index: number) {
	print("LogParam", target, key, index);
}

let mutableDeco = (ctor: defined) => {
	print("mutable", ctor);
};

const ns = {
	deco: (target: defined, key: string) => {
		print("ns.deco", target, key);
	},
};

@Component
@ComponentFactory("hi")
class Decorated {
	@LogProp
	value = 1;
	@LogProp
	static svalue = 2;
	@LogMethod
	method(@LogParam a: number, @LogParam b: string) {
		print(a, b);
	}
	@LogMethod
	static smethod() {}
	constructor(@LogParam x: number) {
		print(x);
	}
}

@mutableDeco
@Component
class MutableDecorated {}

const KEY = "dyn";
class ComputedDecorated {
	@LogMethod
	[KEY]() {}
}

class Factories {
	@LogPropFactory("p")
	@ns.deco
	prop = 3;

	@LogMethodFactory("m")
	@LogMethod
	multi() {}
}

class Base {
	greet() {
		return "base";
	}
}

@Component
export class ErrorBoundary extends Base {
	@LogPropFactory("state")
	currentState = { hasError: false };

	@LogMethodFactory("derived")
	componentDidCatch(message: unknown) {
		print(message, this.greet());
	}
}

print(new Decorated(1), MutableDecorated, ComputedDecorated, Factories, ErrorBoundary, mutableDeco);
