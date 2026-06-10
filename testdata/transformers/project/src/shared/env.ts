import { $env } from "rbxts-transform-env";

export const apiUrl = $env.string("ROTOR_FIXTURE_API_URL", "https://fallback.example");
export const flagEnabled = $env.boolean("ROTOR_FIXTURE_FLAG");
