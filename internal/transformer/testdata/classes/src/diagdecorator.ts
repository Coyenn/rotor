function Deco(ctor: unknown) {
	print(ctor);
}

@Deco
class Decorated {}

print(new Decorated());
