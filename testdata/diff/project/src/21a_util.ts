export const MAX = 100;
export function clamp(x: number) {
	return x > MAX ? MAX : x;
}
export default function describe(x: number) {
	return "v" + x;
}
