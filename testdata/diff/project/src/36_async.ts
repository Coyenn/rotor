async function fetchValue(x: number): Promise<number> {
	const a = await Promise.resolve(x);
	const b = await fetchValue(a);
	return a + b;
}
const asyncArrow = async (n: number) => n * 2;
export async function exported(): Promise<void> {
	await asyncArrow(1);
}
function* gen(): Generator<number, string, undefined> {
	yield 1;
	yield 2;
	return "done";
}
function* gen2() {
	yield;
	const v = yield* gen();
	print(v);
}
export function useGen() {
	for (const x of gen2()) {
		print(x);
	}
}
export function early() {
	return lateAsync();
}
async function lateAsync() {
	return 1;
}
class Foo {
	async work(n: number) {
		return (await Promise.resolve(n)) + 1;
	}
	static async make() {
		return new Foo();
	}
	*counter() {
		yield 1;
		yield 2;
	}
	static *names() {
		yield "a";
	}
	async ["computed " + "key"]() {
		return 1;
	}
}
export const obj = {
	async work(n: number) {
		return (await Promise.resolve(n)) + 1;
	},
	*counter() {
		yield 1;
	},
};
const fexpr = async function (x: number) {
	return x;
};
const genExpr = function* () {
	yield 5;
};
function* outerStmt() {
	yield* gen();
	yield;
}
export async function chainBin(a: () => Promise<number>) {
	const x = 1 + (await a());
	if ((await a()) > 0) {
		await a();
	}
	await a();
	return x;
}
const arrowExprBody = async (n: number) => n + (await Promise.resolve(1));
export function resume() {
	const g = (function* (): Generator<number, void, number> {
		const got = yield 1;
		print(got);
	})();
	g.next();
	g.next(5);
}
export function use() {
	const f = new Foo();
	print(f.work(1), Foo.make(), f.counter(), Foo.names());
	print(fexpr(2), genExpr(), outerStmt(), arrowExprBody(1));
}
