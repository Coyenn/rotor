interface Inner {
	num: number;
	arr?: Array<number>;
}

interface Holder {
	value?: Inner;
	fn?: () => number;
	cb: (() => number) | undefined;
	method?(n: number): number;
	getInner?(): Inner;
	tupleMethod?(): LuaTuple<[number, string]>;
}

declare const holder: Holder | undefined;
declare const exact: Holder;
declare const maybeArr: Array<number> | undefined;
declare const dict: { [k: string]: number } | undefined;
declare const freeFn: (() => number) | undefined;
declare const tupleFn: (() => LuaTuple<[number, string]>) | undefined;
declare const handlers: { [k: string]: (() => number) | undefined };
declare const fnArr: Array<() => number> | undefined;

// optional property access chains
const num = holder?.value?.num;
print(num);

// pure optional access in statement position
holder?.value;

// optional call of a callback property (a.b?.())
exact.cb?.();
const c = exact.cb?.();
print(c);

// optional call of a real method (a.b?.() with isMethod -> _self)
exact.method?.(5);
const m = exact.method?.(7);
print(m);

// optional access AND optional call (a?.b?.() -> double nil check)
holder?.method?.(1);
const dm = holder?.method?.(2);
print(dm);

// optional element access (+1 on array types)
const first = maybeArr?.[0];
print(first);
const v = dict?.["key"];
print(v);

// bare optional call (fn?.())
freeFn?.();
const f = freeFn?.();
print(f);

// LuaTuple results
const t = tupleFn?.();
print(t);
const tm = exact.tupleMethod?.();
print(tm);

// property-call macro inside an optional chain (a?.pop())
maybeArr?.pop();
const popped = maybeArr?.pop();
print(popped);

// macro call as the base of an optional chain
const lookup = new Map<string, Inner>();
const n2 = lookup.get("a")?.num;
print(n2);

// chains ending in calls, methods, and accesses
const deep = holder?.getInner?.()?.num;
print(deep);
print(holder?.value?.arr?.[1]);

// optional element calls (a[b]?.())
handlers["go"]?.();
const h = handlers["stop"]?.();
print(h);
fnArr?.[0]?.();
const fa = fnArr?.[1]?.();
print(fa);

export {};
