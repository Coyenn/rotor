-- Compiled with roblox-ts v3.0.0
local arr = { 1, 2, 3, 4 }
local _binding = arr
local first = _binding[1]
local second = _binding[2]
local _binding_1 = arr
local third = _binding_1[3]
local head = 10
local obj = {
	x = 1,
	y = 2,
	nested = {
		z = 3,
	},
}
local _binding_2 = obj
local x = _binding_2.x
local renamed = _binding_2.y
local _binding_3 = obj
local _binding_4 = _binding_3.nested
local z = _binding_4.z
local _binding_5 = {
	x = 5,
}
local dx = _binding_5.x
if dx == nil then
	dx = 99
end
local missing = _binding_5.missing
if missing == nil then
	missing = 7
end
local a = 0
local b = 0
a, b = b, a
local _binding_6 = obj
a = _binding_6.x
print(first, second, third, head, x, renamed, z, dx, missing, a, b)
return nil
