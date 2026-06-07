export function f1() {
	try {
		print("try");
	} catch (e) {
		print("caught", e);
	}
}
export function f2() {
	try {
		print("try");
	} finally {
		print("finally");
	}
}
export function f4() {
	try {
		print("x");
	} catch {
		print("no binding");
	}
}
export function f5(): number {
	try {
		return 1;
	} catch {
		return 2;
	}
	return 3;
}
export function f6() {
	for (let i = 0; i < 10; i++) {
		try {
			if (i === 5) {
				break;
			}
			print(i);
		} catch {
			continue;
		}
	}
}
export function f7(): number {
	while (true) {
		try {
			return 42;
		} catch {
			break;
		}
	}
	return 0;
}
export function f8(): number {
	try {
		try {
			return 1;
		} catch {}
	} catch {}
	return 2;
}
export function sw(v: number) {
	switch (v) {
		case 1:
			try {
				break;
			} catch {}
		default:
			print("d");
	}
}
export function contOnly() {
	for (let i = 0; i < 3; i++) {
		try {
			continue;
		} catch {}
	}
}
export function finReturn(): number {
	try {
		print("t");
	} finally {
		return 9;
	}
}
export function tcf(): number {
	try {
		return 1;
	} catch (e) {
		print(e);
		return 2;
	} finally {
		print("fin");
	}
	return 3;
}
export function throwInTry() {
	try {
		throw "err";
	} catch (e) {
		print(e);
	}
}
export function catchDestructure() {
	try {
		print("t");
	} catch (e) {
		const { name } = e as { name: string };
		print(name);
	}
}
declare function getTuple(): LuaTuple<[number, string]>;
export function multi(): LuaTuple<[number, string]> {
	try {
		return $tuple(1, "a");
	} catch {}
	return getTuple();
}
export function tupleVal(): LuaTuple<[number, string]> {
	try {
		return getTuple();
	} catch {}
	return getTuple();
}
export function bareReturn() {
	try {
		if (math.random() > 0.5) {
			return;
		}
		print("x");
	} catch {}
}
export async function tryInAsync(): Promise<number> {
	try {
		return await Promise.resolve(1);
	} catch (e) {
		return 2;
	}
}
export function loopInTry() {
	try {
		for (let i = 0; i < 3; i++) {
			if (i === 1) {
				break;
			}
			print(i);
		}
	} catch {}
}
export function fnInTry() {
	try {
		const f = () => {
			return 5;
		};
		print(f());
	} catch {}
}
