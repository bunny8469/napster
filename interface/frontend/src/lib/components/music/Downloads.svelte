<script>
    import { Button } from "$lib/components/ui/button";
    import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "$lib/components/ui/table";
    import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "$lib/components/ui/dropdown-menu";
    import { MoreVertical } from "lucide-svelte";
    import { onMount, onDestroy } from "svelte";

    export let torrents;
    export let handleTorrentOptions;
    
    let internalTorrents = [...torrents]; // make a copy to manage locally

    $: if (torrents) {
        internalTorrents = [...torrents];
    }
    
    // Function to handle download status updates
    function handleDownloadStatus(msg) {
        console.log("Download status:", msg);
        if (msg && msg.filename) {
            internalTorrents = internalTorrents.map(t => {
                if (t.Metadata.file_name === msg.filename) {
                    return {
                        ...t,
                        Status: msg.status // Update the status field
                    };
                }
                return t;
            });
        }
    }
    
    function handleQueue(msg) {
        console.log(msg)
        internalTorrents = [...internalTorrents, {
            Metadata: msg,
            Status: "Queued",
        }];
    }

    function handleUpload(msg) {
        console.log(msg)
        internalTorrents = [...internalTorrents, {
            Metadata: msg,
            Status: "Seeding",
        }];
    }

    onMount(() => {
        // Set up event listener when the component mounts
        window.runtime.EventsOn("download-status", handleDownloadStatus);
        window.runtime.EventsOn("download-queue", handleQueue);
        window.runtime.EventsOn("upload-status", handleUpload);
    });
    
    onDestroy(() => {
        // Clean up event listener when the component is destroyed
        window.runtime.EventsOff("download-status");
        window.runtime.EventsOff("download-queue");
        window.runtime.EventsOff("upload-status");
    });
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
        <TableHead class="text-[#e0e0e0]">Status</TableHead>
        <TableHead class="text-[#e0e0e0]">Size</TableHead>
        <TableHead class="text-[#e0e0e0] w-8"></TableHead>
        </TableRow>
    </TableHeader>
    <TableBody>
        {#each internalTorrents as torrent, i (torrent.Metadata.file_name + '-' + torrent.Status)}
        <TableRow class="border-[#2a2a2a] hover:bg-[#1a1a1a]">
            <TableCell>{i + 1}</TableCell>
            <TableCell>
            <div>
                <p class="font-medium">{torrent.Metadata.file_name}</p>
                <div class="flex items-center gap-1">
                <p class="text-xs text-[#909090]">{torrent.Metadata.artist_name}</p>
                </div>
            </div>
            </TableCell>
            <TableCell>{torrent.Metadata.peers.length}</TableCell>

            <TableCell>
            <!-- Restyled badge for various status types -->
            {#if torrent.Status === "Downloaded" || torrent.Status === "Seeding"}
                <div class="px-2 py-1 text-xs rounded bg-[#1a4d2d] text-[#a9ebc8]">{torrent.Status}</div>
            {:else if torrent.Status === "Fetching Torrent"}
                <div class="px-2 py-1 text-xs rounded bg-[#614a00] text-[#ffe07a]">Fetching Torrent</div>
            {:else if torrent.Status === "Downloading"}
                <div class="px-2 py-1 text-xs rounded bg-[#2c5aa0] text-[#cde1ff]">Downloading</div>
            {:else if torrent.Status === "Paused"}
                <div class="px-2 py-1 text-xs rounded bg-[#61380c] text-[#ffcfa3]">Paused</div>
            {:else}
                <div class="px-2 py-1 text-xs rounded bg-[#575757] text-[#d0d0d0]">{torrent.Status}</div>
            {/if}
            </TableCell>
            <TableCell>{torrent.Metadata.file_size/(1024.0*1024.0) + " MB"}</TableCell>
            <TableCell>
            <DropdownMenu>
                <DropdownMenuTrigger on:click={() => console.log("Trigger clicked")}>
                <Button variant="ghost" size="icon" class="text-[#909090] hover:text-[#4a86e8] hover:bg-[#1a1a1a]">
                    <MoreVertical class="h-4 w-4" />
                </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" class="bg-[#1a1a1a] border-[#333] text-[#e0e0e0]">
                <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("play", torrent)}>
                    Open in Player
                </DropdownMenuItem>
                <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("pause", torrent)}>
                    {torrent.Status === "Downloading" ? "Pause" : "Resume"}
                </DropdownMenuItem>
                <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("toggle-seed", torrent)}>
                    {torrent.Status === "Downloaded" ? "Enable Seeding" : "Stop Seeding"}
                </DropdownMenuItem>
                <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8]" on:click={() => handleTorrentOptions("info", torrent)}>
                    Details
                </DropdownMenuItem>
                <!-- <DropdownMenuItem class="focus:bg-[#333] focus:text-[#4a86e8] text-red-400" on:click={() => handleTorrentOptions("delete", torrent)}>
                    Delete
                </DropdownMenuItem> -->
                </DropdownMenuContent>
            </DropdownMenu>
            </TableCell>
        </TableRow>
        {/each}
    </TableBody>
    </Table>
</div>
</div>