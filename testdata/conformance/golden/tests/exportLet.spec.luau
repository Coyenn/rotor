-- Compiled with roblox-ts v3.0.0
local y = 0
local Foo = {}
do
	local _container = Foo
	_container.x = 1
	y = _container.x
end
local Bar = {}
do
	local _container = Bar
	local plr = {
		ClassName = "Player",
		Name = "Validark",
	}
	local _binding = plr
	local a = _binding.ClassName
	_container.a = a
	local _binding_1 = plr
	_container.b = _binding_1.ClassName
	_container.c = _binding_1.Name
	local _binding_2 = plr
	_container.ClassName = _binding_2.ClassName
	_container.Name = _binding_2.Name
end
return function()
	it("should allow mutatable exports", function()
		expect(Foo.x).to.equal(1)
		Foo.x = 2
		expect(Foo.x).to.equal(2)
		expect(y).to.equal(1)
	end)
	it("should allow destructured let exports", function()
		-- const check
		expect(Bar.a).to.equal("Player")
		expect(Bar.ClassName).to.equal("Player")
		Bar.ClassName = "Attacker"
		expect(Bar.ClassName).to.equal("Attacker")
		expect(Bar.Name).to.equal("Validark")
		expect(Bar.b).to.equal("Player")
		expect(Bar.c).to.equal("Validark")
		Bar.b = "Nope"
		Bar.c = "Osyris"
		expect(Bar.b).to.equal("Nope")
		expect(Bar.c).to.equal("Osyris")
	end)
end
