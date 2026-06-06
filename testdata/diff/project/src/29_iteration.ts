// for-of over Set: single binding `for x in exp do`
declare const tags: Set<string>;
for (const tag of tags) {
	print(tag);
}

// for-of over Map: the [k, v] inline-destructure fast path
declare const scores: Map<string, number>;
for (const [name, score] of scores) {
	print(name, score);
}
// Map plain-id fallback: `for _k, _v in` + `local pair = { _k, _v }`
for (const pair of scores) {
	print(pair[0], pair[1]);
}
// Map assignment-form fast path: `for _binding, _binding in` + writes
let k3 = "";
let v3 = 0;
for ([k3, v3] of scores) {
	print(k3, v3);
}

// for-of over string: string.gmatch(exp, utf8.charpattern)
const word = "héllo";
for (const ch of word) {
	print(ch);
}

// $range without step
for (const i of $range(1, 5)) {
	print(i);
}
// $range with a literal step
for (const j of $range(10, 2, 2)) {
	print(j);
}
// $range with a non-literal step expression (`or 1` defaulting)
declare const stepValue: number;
for (const n of $range(0, 10, stepValue)) {
	print(n);
}

// for-of over a declared generator object: .next()/.done/.value protocol
declare const gen: Generator<number, void, unknown>;
for (const g of gen) {
	print(g);
}

// for-of over IterableFunction<T>: `for x in exp do`
declare const iter: IterableFunction<number>;
for (const v of iter) {
	print(v);
}

// IterableFunction<LuaTuple<T>>: inline array-pattern fast path
declare const pairsIter: IterableFunction<LuaTuple<[key: string, value: number]>>;
for (const [pk, pv] of pairsIter) {
	print(pk, pv);
}
// plain-id over a labeled tuple: arity introspection names temps from labels
for (const entry of pairsIter) {
	print(entry[0], entry[1]);
}
// plain-id over an unlabeled tuple: temps named `_element`
declare const dualIter: IterableFunction<LuaTuple<[number, string]>>;
for (const duo of dualIter) {
	print(duo[0], duo[1]);
}
// rest-element tuple: unknown arity, while-true protocol
declare const restIter: IterableFunction<LuaTuple<[string, ...number[]]>>;
for (const packed of restIter) {
	print(packed[0]);
}

// destructuring FROM a Map: `local _k, _v = next(map[, lastK])`
const [[ka, va], [kb, vb]] = scores;
print(ka, va, kb, vb);

// destructuring FROM a Set: `next(set[, lastValue])` continuation
const [firstTag, secondTag] = tags;
print(firstTag, secondTag);

// destructuring FROM a string: gmatch matcher (hole still advances)
const [c1, , c3] = word;
print(c1, c3);

// destructuring FROM an IterableFunction: `iter()` per element
const [n1, n2] = iter;
print(n1, n2);

// destructuring FROM a generator object: `gen.next().value`
const [g1, g2] = gen;
print(g1, g2);
