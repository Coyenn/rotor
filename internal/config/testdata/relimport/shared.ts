export const CREATOR = { type: "user", id: 99 } as const;

const UNIVERSES: Record<string, number> = { dev: 777, prod: 888 };

export function universeFor(env: string): number {
	return UNIVERSES[env] ?? 0;
}
