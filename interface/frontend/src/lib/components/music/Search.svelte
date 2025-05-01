<script>
    import { Button } from "$lib/components/ui/button";
    import { Input } from "$lib/components/ui/input";
    import { Download } from "lucide-svelte";
    import { DownloadFile } from "$lib/wailsjs/go/main/App";
    
    export let searchQuery;
    export let searchResults;
    export let handleSearch;
  </script>

<div class="bg-[#121212] border border-[#2a2a2a] rounded-md p-4 h-min">
    <div class="flex gap-2 mb-4">
    <Input 
        placeholder="Search for songs, artists..." 
        class="bg-[#1a1a1a] border-[#333] text-[#e0e0e0] focus:border-[#4a86e8] focus-visible:ring-0 focus-visible:ring-offset-0" 
        bind:value={searchQuery}
        on:keydown={(e) => {
            if (e.key === "Enter") handleSearch();
        }}        
    />
    <Button class="bg-[#4a86e8] hover:bg-[#6a9ae8] text-white" on:click={handleSearch}>Search</Button>
    </div>
    
    <!-- Search results grid -->
    <div class="mt-4 h-80 overflow-y-auto custom-scrollbar"> <!-- Increased height from h-64 to h-80 -->
    {#if searchQuery || searchResults.length > 0}
        <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        {#each searchResults as song}
            <div class="song-tile flex items-center justify-between p-3 bg-[#1a1a1a] rounded-md hover:bg-[#222] transition-colors">
            <div class="flex items-center gap-3">
                <div class="w-12 h-12 bg-gradient-to-br from-[#1a1a1a] to-[#2a2a2a] rounded-md flex items-center justify-center text-xs text-white">
                <div class="bg-gradient-to-br from-[#2c5aa0] to-[#4a86e8] bg-clip-text text-transparent text-xl">♪</div>
                </div>
                <div>
                <p class="font-medium text-white">{song.name}</p>
                <div class="flex items-center gap-1">
                    <p class="text-xs text-[#909090]">{song.artist}</p>
                    <!-- {#if song.hasVideo}
                    <Badge variant="outline" class="text-xs py-0 h-4 bg-[#1a1a1a] border-[#333] text-[#ccc]">Video</Badge>
                    {/if} -->
                </div>
                <p class="text-xs text-[#909090]">{song.peers} Peers</p>
                </div>
            </div>
            <Button variant="ghost" size="icon" class="text-[#909090] hover:text-[#4a86e8] hover:bg-[#1a1a1a]" on:click={() => DownloadFile(song.name)}>
                <Download class="h-4 w-4" />
            </Button>
            </div>
        {/each}
        </div>
    {:else}
        <p class="text-[#909090] text-center">Enter a search term to find music</p>
    {/if}
    </div>
</div>