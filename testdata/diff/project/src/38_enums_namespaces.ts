enum Fruit {
	Apple,
	Banana,
	Cherry,
}
enum Mixed {
	A = 5,
	B,
	C = 10,
}
enum Color {
	Red = "red",
	Green = "green",
}
enum Hetero {
	Num = 1,
	Str = "str",
}
const base = 10;
enum Computed {
	X = base * 2,
	Y = "y".size(),
}
const enum Direction {
	Up,
	Down,
}
declare enum Ambient {
	A,
}
export enum Exported {
	E1,
	E2,
}
enum Weird {
	["a b"] = 1,
	ok = 2,
}
export function early(): number {
	return E.A;
}
enum E {
	A,
}
export function earlyStr(): string {
	return SE.S;
}
enum SE {
	S = "s",
}
export function uses() {
	print(Fruit.Apple, Fruit[1], Mixed.B, Color.Red, Hetero.Str, Computed.X, Computed.Y, Weird.ok);
	print(Direction.Up, Direction.Down);
	print(Exported.E2);
}
namespace Outer {
	export const value = 1;
	export let mut = 2;
	export function fn() {
		return value;
	}
	export function bump() {
		mut += 1;
		return mut;
	}
	const hidden = 3;
	export namespace Inner {
		export const deep = hidden;
	}
}
export function useNs() {
	print(Outer.value, Outer.fn(), Outer.Inner.deep, Outer.mut, Outer.bump());
	Outer.mut = 5;
}
namespace NoExports {
	const x = 1;
	print(x);
}
namespace AB.CD {
	export const v = 1;
}
export function useAB() {
	print(AB.CD.v);
}
declare namespace AmbientNs {
	const x: number;
}
namespace TypesOnly {
	export type T = number;
}
export function earlyNs(): number {
	return NS.v;
}
namespace NS {
	export const v = 7;
}
namespace HasEnum {
	export enum Inner2 {
		X,
	}
	export const c = Inner2.X;
}
export function useHasEnum() {
	print(HasEnum.c);
}
export namespace ExpNs {
	export const e = 1;
	export const p = 1,
		q = 2;
}
