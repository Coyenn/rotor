import { BASE, scale } from "./24a_shared";
import { Pair } from "./24a_shared";
import { DOUBLE, sum, rescale } from "./24b_middle";

const p: Pair = { first: 3, second: 4 };
print(BASE, scale(2), DOUBLE);
print(sum(p), rescale(p.second));

const fns = new Array<() => Map<string, number>>();
for (let i = 0; i < 3; i++) {
	fns[i] = () => {
		const m = new Map<string, number>();
		print("make", i);
		return m;
	};
}
print(fns[0](), fns[2]());
export {};
