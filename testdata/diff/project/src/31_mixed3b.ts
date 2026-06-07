// Phase 3b adversarial mix: macro tables, optional chaining, and the full
// iteration builders interacting with each other — not in isolation.

interface Entry {
	tags: Set<string>;
	weights?: Map<string, number>;
}

declare const registry: Map<string, Entry> | undefined;
declare function getFallback(): Map<string, Entry>;

// ?? joining an optional Map with a Map-returning call, used as a for-of source
const source = registry ?? getFallback();

// for-of over a Map with [k, v] destructuring; optional-chained macros + ?? in body
const lines = new Array<string>();
for (const [name, entry] of source) {
	const weight = entry.weights?.get(name) ?? entry.tags.size();
	// string macro on a template-literal result, pushed via an Array macro
	lines.push(`${name}:${weight}`.upper());
}

// String/Array macro chain: join -> split -> map -> filter -> join
const slug = lines
	.join(",")
	.split(",")
	.map((part) => part.lower())
	.filter((part) => part.size() > 3)
	.join("-");
print(slug);

// Map macro inside an optional chain, its result optional-accessed into a Set macro
const probed = registry?.get("alpha")?.tags.has("hot");
print(probed);

// optional chain on a macro RESULT (pop -> string | undefined -> string macro)
const popped = lines.pop()?.sub(1, 4);
print(popped);

// assert narrows away undefined; the follow-up macro call is non-optional
assert(registry, "registry missing");
print(registry.size());

// typeIs guard unlocking string macros on an unknown, chained into ArrayLike.size
declare const payload: unknown;
if (typeIs(payload, "string")) {
	print(payload.split(" ").size());
}

// destructuring FROM a Set feeding $tuple, consumed inside a Map for-of with
// an omitted key binding
function firstTwo(tags: Set<string>): LuaTuple<[string | undefined, string | undefined]> {
	const [a, b] = tags;
	return $tuple(a, b);
}
for (const [, entry] of source) {
	const [first, second] = firstTwo(entry.tags);
	print(first, second ?? "none");
}

// $range with a macro-heavy body: chained Map.set, template literal + rep
const counts = new Map<number, string>();
for (const i of $range(1, 5, 2)) {
	counts.set(i, `#${i}`.rep(i)).set(-i, "neg");
}
counts.forEach((v, k) => print(k, v.size()));

// nested binding pattern (default + hole) over an IterableFunction of LuaTuples,
// with macros on every piece
declare const pairsIter: IterableFunction<LuaTuple<[string, Array<number>]>>;
for (const [key, [head = 0, , third]] of pairsIter) {
	print(key.upper(), head, third ?? head);
}

// generator object iteration; LuaTuple-returning string macro in the body
declare function wordStream(): Generator<string, void, unknown>;
for (const word of wordStream()) {
	const [start] = word.find("o");
	print(start ?? -1);
}

export {};
