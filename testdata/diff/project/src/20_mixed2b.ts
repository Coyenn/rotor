// declared before the exported function: rbxtsc still returns the function
// first (binder binds FunctionDeclarations before other statements)
export const VERSION = 3;
function makeAdder(base: number) {
	return (x: number) => base + x;
}
function makeCounter(start: number) {
	let count = start;
	return () => {
		count += 1;
		return count;
	};
}
export function classify(items: Array<number>) {
	let evens = 0;
	let odds = 0;
	for (const item of items) {
		switch (item % 2) {
			case 0:
				evens += 1;
				break;
			default:
				odds += 1;
		}
	}
	return { evens, odds };
}
function bindAll(...nums: Array<number>) {
	return (extra: number) => {
		let total = extra;
		for (const n of nums) {
			total += n;
		}
		return total;
	};
}
function withDefaults(a: number, b = a + 1, f = (n: number) => n * a) {
	return f(b);
}
function pickLabel(kind: string) {
	const { label } = { label: kind === "x" ? "extra" : "normal" };
	return () => label;
}
const { evens, odds } = classify([1, 2, 3, 4, 5]);
const add5 = makeAdder(5);
const tick = makeCounter(10);
const pairList: Array<Array<number>> = [[1, 10], [2, 20], [3, 30]];
const fns: Array<() => number> = [];
let slot = 0;
for (const [k, v] of pairList) {
	fns[slot] = () => k * 100 + v;
	slot += 1;
}
const sum3 = bindAll(1, 2, 3);
print(add5(10), evens, odds, withDefaults(2), withDefaults(3, 7), sum3(4));
print(fns[0](), fns[1](), fns[2](), tick(), tick(), pickLabel("x")(), pickLabel("y")());
