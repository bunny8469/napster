<script>
    import { Music4, Settings } from "lucide-svelte";
    import { Button } from "$lib/components/ui/button";
    import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "$lib/components/ui/dropdown-menu";

    export let stopSeeding;

</script>

<header class="border-b border-[#2a2a2a] p-4 flex justify-between items-center bg-[#121212]">
    <div class="flex items-center gap-2">
      <span class="text-xl font-bold text-[#4a86e8]"> <Music4 class="inline-block mr-2"/> Napster</span>
    </div>
    <div class="flex gap-4 items-center">
      <Button variant="ghost" class="text-[#e0e0e0] hover:bg-[#1a1a1a] hover:text-[#4a86e8]" on:click={stopSeeding}>Stop Seeding</Button>
      <Button variant="ghost" class="text-[#e0e0e0] hover:bg-[#1a1a1a] hover:text-[#4a86e8]">Option #2</Button>
      <DropdownMenu>
        <DropdownMenuTrigger>
          <Button variant="ghost" size="icon" class="text-[#e0e0e0] hover:bg-[#1a1a1a] hover:text-[#4a86e8]">
            <Settings class="h-5 w-5" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" class="bg-[#1a1a1a] border-[#333] text-[#e0e0e0]">
          <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => alert("Opening preferences")}>
            Preferences
          </DropdownMenuItem>
          <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => alert("Managing downloads")}>
            Download Settings
          </DropdownMenuItem>
          <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => alert("Network configuration")}>
            Network Settings
          </DropdownMenuItem>
          <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => alert("About this application")}>
            About
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
</header>