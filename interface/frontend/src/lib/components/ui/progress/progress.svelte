<script>
	import { Progress as ProgressPrimitive } from "bits-ui";
	import { cn } from "$lib/utils.js";
	let className = undefined;
	export let max = 100;
	export let value = undefined;
	export { className as class };
</script>

<ProgressPrimitive.Root
	class={cn("bg-primary/20 relative h-2 w-full overflow-hidden rounded-full", className)}
	{...$$restProps}
>
	<div
		class="bg-primary h-full w-full flex-1 transition-all"
		style={`transform: translateX(-${100 - (100 * (value ?? 0)) / (max ?? 1)}%)`}
	></div>
</ProgressPrimitive.Root>
