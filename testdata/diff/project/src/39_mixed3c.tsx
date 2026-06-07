// Phase 3c interactions: decorated class components rendering JSX, spread
// props through table.clone, enum-keyed Map iteration feeding children,
// generators (with yield* inside try) spread into JSX, async methods with
// try/await + break/continue rerouting inside loops, ??= on class fields,
// namespace-exported components as JSX tags, call spread into variadics.
import React from "@rbxts/react";

enum Slot {
	Head,
	Body,
	Legs,
}

const labels = new Map<Slot, string>([
	[Slot.Head, "Head"],
	[Slot.Body, "Body"],
	[Slot.Legs, "Legs"],
]);

function Card(props: { title: string; order: number }) {
	return <textlabel Text={props.title} LayoutOrder={props.order} />;
}

namespace UI {
	export function Badge(props: { text: string }) {
		return <textlabel Text={props.text} />;
	}
	export const DEFAULT_TEXT = "badge";
}

function Tag(label: string) {
	return (ctor: defined) => print("Tag", label, ctor);
}

function LogMethod(target: defined, key: string, descriptor: { value: unknown }) {
	print("LogMethod", key, descriptor);
}

function* extras(): Generator<React.ReactElement, void, undefined> {
	yield <UI.Badge key="extra" text={UI.DEFAULT_TEXT} />;
}

function* cards(): Generator<React.ReactElement, void, undefined> {
	let order = 0;
	try {
		for (const [slot, title] of labels) {
			if (slot === Slot.Legs) {
				break;
			}
			order += 1;
			yield <Card key={`gen${order}`} title={title} order={order} />;
		}
		yield* extras();
	} catch (e) {
		print("generator failed", e);
	}
}

interface PanelProps {
	header: string;
	accent?: number;
}

@Tag("panel")
@React.ReactComponent
export class Panel extends React.Component<PanelProps> {
	cache?: Map<Slot, string>;
	static instances = 0;

	constructor(props: PanelProps) {
		super(props);
		Panel.instances += 1;
	}

	@LogMethod
	async load(sources: Array<() => Promise<string>>) {
		let found: string | undefined;
		for (const src of sources) {
			try {
				const value = await src();
				found ??= value;
				if (value === "stop") {
					break;
				}
			} catch (e) {
				print("load failed", e);
				continue;
			}
		}
		return found;
	}

	render() {
		const entries = (this.cache ??= labels);
		const children = new Array<React.ReactElement>();
		for (const [slot, title] of entries) {
			children.push(<Card key={tostring(slot)} title={title} order={slot} />);
		}
		const shared = { title: this.props.header, order: this.props.accent ?? 0 };
		const orders = [...labels].map(([slot]) => slot as number);
		return (
			<frame LayoutOrder={math.max(0, ...orders)}>
				<Card {...shared} />
				<Card {...shared} order={Panel.instances} />
				{children}
				{[...cards()]}
				<textbutton
					Text={Slot[Slot.Body]}
					Event={{
						MouseButton1Click: async () => {
							try {
								print(await this.load([async () => "stop"]));
							} finally {
								print("handled");
							}
						},
					}}
				/>
			</frame>
		);
	}
}

export const tree = (
	<screengui ResetOnSpawn={false}>
		<Panel header="H" accent={2} />
		<UI.Badge text="outer" />
	</screengui>
);
