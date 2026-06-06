function add(a: number, b: number) {
	return a + b;
}
function greet(name: string, punct = "!") {
	return "hi " + name + punct;
}
function sum(...nums: Array<number>) {
	let total = 0;
	for (const n of nums) {
		total += n;
	}
	return total;
}
function isEven(n: number): boolean {
	return n === 0 ? true : isOdd(n - 1);
}
function isOdd(n: number): boolean {
	return n === 0 ? false : isEven(n - 1);
}
export function double(x: number) {
	return x * 2;
}
print(add(1, 2), greet("bob"), greet("ann", "?"), sum(1, 2, 3), isEven(4), double(21));
