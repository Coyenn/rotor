// object spread
const base = { x: 1, y: 2 };
const extra = { z: 3 };
function getObj() {
	return { a: "k", b: 1 };
}
declare const maybe: { x: number } | undefined;
declare const props: { event?: { cb: number } };

const spreadOnly = { ...base };
const spreadFirst = { ...base, z: 3 };
const spreadLast = { z: 3, ...base };
const spreadTwice = { ...base, ...extra };
const spreadMaybe = { ...maybe, q: 1 };
const spreadOptionalProp = { a: 1, ...props.event };
const spreadCall = { ...getObj(), b: 2 };
const computedAfterSpread = { ...base, [getObj().a]: 9 };
print(spreadOnly, spreadFirst, spreadLast, spreadTwice, spreadMaybe, spreadOptionalProp, spreadCall, computedAfterSpread);

// array spread
const arr1 = [1, 2];
const arr2 = [3, 4];
declare const nset: Set<number>;
declare const nmap: Map<string, number>;
declare const str: string;
const a1 = [...arr1];
const a2 = [...arr1, 5];
const a3 = [5, ...arr1];
const a4 = [...arr1, ...arr2];
const a5 = [...arr1, 5, ...arr2, 6];
const a6 = [...nset];
const a7 = [...nmap];
const a8 = [...str];
const a9 = [...arr1, getObj().b];
print(a1, a2, a3, a4, a5, a6, a7, a8, a9);

// call spread
function takeNums(...nums: Array<number>) {
	return nums.size();
}
print(takeNums(...arr1));
print(takeNums(1, ...arr1));
print(takeNums(...nset));

// logical assignments
let v: number | undefined;
v ??= 1;
const holder: { p?: number } = {};
function f() {
	return 5;
}
holder.p ??= f();
const arrIdx: Array<number | undefined> = [];
let i = 0;
arrIdx[i] ??= f();
function get(): { p?: number } {
	return holder;
}
get().p ??= f();
let s: string | undefined = "x";
s &&= "y";
s ||= "z";
let n = 0;
n ||= 7;
n &&= 8;
const useAsExpr = (v ??= 2);
print(v, holder.p, arrIdx[i], s, n, useAsExpr, i);
