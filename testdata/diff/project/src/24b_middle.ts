import { BASE, scale, Pair } from "./24a_shared";

export { scale as rescale } from "./24a_shared";

export const DOUBLE = BASE * 2;
export function sum(p: Pair) {
	return scale(p.first) + p.second;
}
