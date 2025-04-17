<script>
    import { Button } from "$lib/components/ui/button";
    import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "$lib/components/ui/dropdown-menu";
    import { MoreVertical } from "lucide-svelte";

    export let libtorrents;
    // export let handleTorrentOptions;
  </script>
  
  <div class="bg-[#121212] border border-[#2a2a2a] rounded-md p-4 h-min">
    <h2 class="text-xl font-bold mb-4 text-white">Library</h2>
    <div class="fixed-height-library custom-scrollbar">
      {#each libtorrents as torrent, i}
        <div class="flex items-center gap-3 p-3 mb-2 hover:bg-[#1a1a1a] rounded-md cursor-pointer transition-colors duration-200">
          <span class="text-[#909090] w-6 text-center">{i + 1}</span>
          <div class="w-10 h-10 bg-gradient-to-br from-[#1a1a1a] to-[#2a2a2a] rounded-md flex items-center justify-center text-xs text-white">
            <div class="bg-gradient-to-br from-[#2c5aa0] to-[#4a86e8] bg-clip-text text-transparent text-lg">♪</div>
          </div>
          <div class="flex-1">
            <p class="font-medium text-white">{torrent.Metadata.file_name}</p>
            <p class="text-xs text-[#909090]">{torrent.Metadata.artist_name}</p>
          </div>
          <!-- <DropdownMenu>
            <DropdownMenuTrigger>
              <Button variant="ghost" size="icon" class="text-[#909090] hover:text-[#4a86e8] hover:bg-[#1a1a1a]">
                <MoreVertical class="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" class="bg-[#1a1a1a] border-[#333] text-[#e0e0e0]">
              <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("play", torrent)}>
                Play
              </DropdownMenuItem>
              <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("info", torrent)}>
                Details
              </DropdownMenuItem>
              <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("stop", torrent)}>
                Stop Seeding
              </DropdownMenuItem>
              <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8] text-red-400" on:click={() => handleTorrentOptions("delete", torrent)}>
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu> -->
        </div>
      {/each}
    </div>
  </div>