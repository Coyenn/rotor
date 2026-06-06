const square = (x: number) => x * x;
const clamp = (x: number, lo: number, hi: number) => {
	if (x < lo) {
		return lo;
	}
	return x > hi ? hi : x;
};
let counter = 0;
const bump = () => {
	counter += 1;
	return counter;
};
const ops = {
	twice: (x: number) => x * 2,
	apply: (f: (n: number) => number, v: number) => f(v),
};
print(square(5), clamp(15, 0, 10), bump(), bump(), ops.twice(4), ops.apply(square, 6));
