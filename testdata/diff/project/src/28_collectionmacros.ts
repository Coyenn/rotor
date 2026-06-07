function getSet(): Set<string> {
	return new Set(["a", "b"]);
}

function getMap(): Map<string, number> {
	return new Map([["k", 1]]);
}

// Set: READONLY_SET_MAP_SHARED (size loop-count, has, isEmpty)
const set = new Set<number>([1, 2, 3]);
print(set.size(), set.isEmpty(), set.has(2));
print(getSet().size(), getSet().isEmpty(), getSet().has("a"));

// Set.add: statement vs value (chaining) position
set.add(4);
const chained = set.add(5).add(6);
print(chained);

// SET_MAP_SHARED delete: statement vs value position
set.delete(1);
const removed = set.delete(2);
print(removed);
print(getSet().delete("a"));

// Set.forEach: callback receives (value, value2, set) — the value TWICE
set.forEach((value, value2, container) => print(value, value2, container.size()));
set.forEach((v) => print(v));
getSet().forEach((v) => print(v));

// ReadonlySet
const rs: ReadonlySet<string> = new ReadonlySet(["x", "y"]);
print(rs.size(), rs.isEmpty(), rs.has("x"));
rs.forEach((a, b) => print(a, b));

// SET_MAP_SHARED clear
set.clear();

// Map: shared methods + get/set
const map = new Map<string, number>([
	["a", 1],
	["b", 2],
]);
print(map.size(), map.isEmpty(), map.has("a"), map.get("a"));
print(getMap().size(), getMap().get("k"));

// Map.set: statement vs value (chaining) position
map.set("c", 3);
const mchained = map.set("d", 4).set("e", 5);
print(mchained);

// Map delete / clear
map.delete("a");
const mremoved = map.delete("b");
print(mremoved);

// Map.forEach: callback receives (value, key, map) — VALUE first
map.forEach((value, key, container) => print(key, value, container.size()));
map.forEach((v) => print(v));
getMap().forEach((v, k) => print(k, v));
map.clear();

// ReadonlyMap
const rm: ReadonlyMap<string, number> = new ReadonlyMap([["k", 9]]);
print(rm.get("k"), rm.has("k"), rm.size(), rm.isEmpty());
rm.forEach((v, k) => print(k, v));

// WeakSet/WeakMap reuse the Set/Map method symbols through inheritance
const ws = new WeakSet<{ id: number }>();
const tag = { id: 1 };
ws.add(tag);
print(ws.has(tag));
const wm = new WeakMap<{ id: number }, number>();
wm.set(tag, 5);
print(wm.get(tag));

// Promise: identifier macro (TS.Promise) + then -> andThen; cancel is a REAL
// runtime method (not macro-only), so it emits a plain method call
const promise = new Promise<number>((resolve) => resolve(1));
const p2 = promise.then((v) => v + 1);
print(p2);
promise.then((v) => print(v));
promise.cancel();
const p3 = Promise.resolve(7);
print(p3);

// CALL_MACROS
// assert: boolean condition (no truthiness expansion) + number|undefined
// condition with message (full 0/NaN/nil expansion)
assert(set.has(3) || true);
function check(x?: number) {
	assert(x, "no x");
	return x;
}
print(check(3));

// typeOf
print(typeOf(5), typeOf("x"), typeOf(set));

// typeIs: primitive Luau type string -> type(); Roblox type -> typeof()
const u: unknown = 4 as unknown;
if (typeIs(u, "number")) {
	print(u + 1);
}
if (typeIs(u, "Vector3")) {
	print(u.X);
}

// classIs: ClassName comparison
const inst: Instance = new Instance("Folder");
if (classIs(inst, "Part")) {
	print(inst.Size);
}

// identity compiles to its argument
print(identity<number>(7));
const idArr = identity([1, 2]);
print(idArr);

// $tuple: multi-value return
function pair(): LuaTuple<[number, string]> {
	return $tuple(1, "a");
}
const [num, str] = pair();
print(num, str);

// $getModuleTree: { root, { "rest", "of", "path" } }
print($getModuleTree("./24a_shared"));

export {};
