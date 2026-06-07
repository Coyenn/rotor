import React from "@rbxts/react";

// self-closing host element, props incl. implicit-true
export const a = <frame BackgroundTransparency={0.5} Visible />;

// element with props AND children (no nil separator)
export const b = (
	<screengui ResetOnSpawn={false}>
		<frame Visible={true} />
		<textlabel Text="hi" />
	</screengui>
);

// fragment with children
export const c = (
	<>
		<frame />
		<frame />
	</>
);

// empty fragment
export const empty = <></>;

// && and ternary expression children
declare const cond: boolean;
export const d = (
	<frame>
		{cond && <frame />}
		{cond ? <frame /> : <textlabel />}
	</frame>
);

// key is a plain prop in the React world
function Item(props: { text: string }) {
	return <textlabel Text={props.text} />;
}
export const f = (
	<frame key="container">
		<Item key="one" text="1" />
		<Item key="two" text="2" />
	</frame>
);

// Event/Change tables are ordinary props
export const g = (
	<textbutton
		Event={{ MouseButton1Click: () => print("click") }}
		Change={{ AbsoluteSize: () => print("size") }}
	/>
);

// .map children — the array is ONE child
declare const list: Array<string>;
export const h = <frame>{list.map(s => <textlabel key={s} Text={s} />)}</frame>;

// attrs + children together
declare function getEl(): React.ReactElement;
export const j = <frame Visible={true}>{getEl()}</frame>;

// the providers.tsx shape
declare const children: React.ReactElement;
export const k = <>{children}</>;
