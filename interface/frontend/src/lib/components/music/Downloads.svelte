<script>
    import { Button } from "$lib/components/ui/button";
    import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "$lib/components/ui/table";
    import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "$lib/components/ui/dropdown-menu";
    import { MoreVertical } from "lucide-svelte";

    export let torrents;
    export let handleTorrentOptions;
  </script>
  
<div class="md:col-span-2 bg-[#121212] border border-[#2a2a2a] rounded-md p-4">
<h2 class="text-xl font-bold mb-4 text-white">Downloads</h2>
<div class="fixed-height-container table-fixed-scrollbar custom-scrollbar">
    <Table>
    <TableHeader>
        <TableRow class="border-[#2a2a2a]">
        <TableHead class="text-[#e0e0e0]">#</TableHead>
        <TableHead class="text-[#e0e0e0]">Title</TableHead>
        <TableHead class="text-[#e0e0e0]">Peers</TableHead>
        <TableHead class="text-[#e0e0e0]">Progress</TableHead>
        <TableHead class="text-[#e0e0e0]">Status</TableHead>
        <TableHead class="text-[#e0e0e0]">Size</TableHead>
        <TableHead class="text-[#e0e0e0]">Speed</TableHead>
        <TableHead class="text-[#e0e0e0] w-8"></TableHead>
        </TableRow>
    </TableHeader>
    <TableBody>
        {#each torrents as torrent, i}
        <TableRow class="border-[#2a2a2a] hover:bg-[#1a1a1a]">
            <TableCell>{i + 1}</TableCell>
            <TableCell>
            <div>
                <p class="font-medium">{torrent.name}</p>
                <div class="flex items-center gap-1">
                <p class="text-xs text-[#909090]">{torrent.artist}</p>
                <!-- {#if torrent.hasVideo}
                    <Badge variant="outline" class="text-xs py-0 h-4 bg-[#1a1a1a] border-[#333] text-[#ccc]">Music Video</Badge>
                {/if} -->
                </div>
            </div>
            </TableCell>
            <TableCell>{torrent.peers}</TableCell>
            <TableCell>
            <div class="w-24">
                <div class="custom-progress-bar">
                <div class="custom-progress-value" style="width: {torrent.progress}%"></div>
                </div>
                <span class="text-xs text-[#909090] mt-1">{torrent.progress}%</span>
            </div>
            </TableCell>
            <TableCell>
            <!-- Restyled badge without border for Complete status -->
            {#if torrent.status === "Complete"}
                <div class="px-2 py-1 text-xs rounded bg-[#1a4d2d] text-[#a9ebc8]">Complete</div>
            {:else}
                <div class="px-2 py-1 text-xs rounded bg-[#2c5aa0] text-[#cde1ff]">Downloading</div>
            {/if}
            </TableCell>
            <TableCell>{torrent.size}</TableCell>
            <TableCell>{torrent.speed}</TableCell>
            <TableCell>
            <DropdownMenu>
                <DropdownMenuTrigger>
                <Button variant="ghost" size="icon" class="text-[#909090] hover:text-[#4a86e8] hover:bg-[#1a1a1a]">
                    <MoreVertical class="h-4 w-4" />
                </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" class="bg-[#1a1a1a] border-[#333] text-[#e0e0e0]">
                <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("play", torrent)}>
                    Play
                </DropdownMenuItem>
                <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("pause", torrent)}>
                    {torrent.status === "Downloading" ? "Pause" : "Resume"}
                </DropdownMenuItem>
                <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("stop", torrent)}>
                    Stop Seed
                </DropdownMenuItem>
                <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("info", torrent)}>
                    Details
                </DropdownMenuItem>
                <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8] text-red-400" on:click={() => handleTorrentOptions("delete", torrent)}>
                    Delete
                </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>
            </TableCell>
        </TableRow>
        {/each}
    </TableBody>
    </Table>
</div>
</div>