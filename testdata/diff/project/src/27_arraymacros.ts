function getArr(): Array<number> {
	return [3, 1, 2];
}

const arr = [1, 2, 3];

// ReadonlyArray
print(arr.isEmpty(), ([] as Array<number>).isEmpty());

// join: number elements (no tostring pass), default + custom separators
print(arr.join(), arr.join("-"));
const sep = ":";
print(arr.join(sep));
// join with the tostring pre-pass (boolean elements)
const flags = [true, false];
print(flags.join());
print(getArr().map((v) => v > 1).join("|"));

// move / includes / indexOf
const dest = [0, 0, 0, 0];
arr.move(0, 2, 1, dest);
print(dest);
print(arr.includes(2), arr.includes(2, 1));
print(arr.indexOf(3), arr.indexOf(3, 1), getArr().indexOf(9));

// every / some
print(arr.every((v, i) => v > i));
print(arr.some((v, i, a) => v === a.size() - i));

// forEach: statement vs value position
arr.forEach((v) => print(v));
const fe = arr.forEach((v, i) => print(v, i));
print(fe);

// map / mapFiltered / filterUndefined / filter
const doubled = arr.map((v, i) => v * 2 + i);
print(doubled);
const mf = arr.mapFiltered((v) => (v > 1 ? v * 10 : undefined));
print(mf);
const holes: Array<number | undefined> = [1, undefined, 3];
print(holes.filterUndefined());
print(arr.filter((v) => v % 2 === 1));

// reduce: no initialValue vs initialValue
print(arr.reduce((a, b) => a + b));
print(arr.reduce((acc, v, i) => acc + v * i, 10));

// find / findIndex
print(arr.find((v) => v === 2));
print(arr.findIndex((v) => v === 2));

// complex base: header-exempt `local _exp = ...` push
print(getArr().map((v) => v + 1));

// Array
const list = [5, 6];
list.push(7);
list.push(8, 9);
const newLen = list.push(10);
print(newLen, list.push());
list.push();
list.pop();
const popped = list.pop();
print(popped);
list.shift();
const shifted = list.shift();
print(shifted, list);
list.unshift(4);
const ulen = list.unshift(2, 3);
print(ulen);
list.insert(1, 99);
list.remove(0);
const removed = list.remove(1);
print(removed);
list.unorderedRemove(0);
const ur = list.unorderedRemove(1);
print(ur);
list.sort();
list.sort((a, b) => a > b);
const sorted = list.sort((a, b) => a < b);
print(sorted);
list.clear();
print(list.isEmpty());

export {};
