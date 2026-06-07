// classes: declarations, expressions, inheritance, statics, abstract,
// parameter properties, computed names, hoisting, toString/__tostring
export class Animal {
	name: string;
	legs = 4;
	constructor(name: string) {
		this.name = name;
	}
	walk(dist: number) {
		print(this.name, dist);
	}
	describe(this: void) {
		print("callback-style method");
	}
	["with space"]() {
		return 1;
	}
	static create() {
		return new Animal("a");
	}
	static VERSION = 1;
}

class Point {
	constructor(
		public x: number,
		public readonly y = 10,
	) {}
	toString() {
		return `(${this.x}, ${this.y})`;
	}
	lengthSquared() {
		return this.x * this.x + this.y * this.y;
	}
}

class Point3 extends Point {
	constructor(
		x: number,
		public z: number,
	) {
		print("pre-super");
		super(x);
		print("post-super", this.z);
	}
	lengthSquared() {
		return super.lengthSquared() + this.z * this.z;
	}
	manhattan() {
		return super["lengthSquared"]();
	}
}

class Inherited extends Point {}

abstract class Shape {
	abstract area(): number;
	describe() {
		return "shape area: " + tostring(this.area());
	}
}

abstract class NamedShape extends Point {
	abstract label(): string;
}

class Square extends Shape {
	constructor(private side: number) {
		super();
	}
	area() {
		return this.side * this.side;
	}
}

const Runtime = class Inner {
	tag = "inner";
	self() {
		return Inner;
	}
};

const Anon = class {
	value() {
		return 42;
	}
};

class Registry {
	static entries = 0;
	static {
		Registry.entries = 1;
		print(this.entries);
	}
}

const KEY = "dynamic";

class Computed {
	[KEY]() {
		return "computed";
	}
}

function early() {
	return new Hoisted();
}

class Hoisted {
	when = "later";
}

export default class {
	run() {
		return "default";
	}
}

const animal = Animal.create();
animal.walk(Animal.VERSION);
animal.describe();
print(animal["with space"]());
const p3 = new Point3(1, 2);
print(tostring(p3), p3.lengthSquared(), p3.manhattan(), new Inherited(5));
print(new Square(3).describe(), NamedShape);
print(new Runtime().self(), new Anon().value(), Registry.entries);
print(new Computed()[KEY](), early().when);
