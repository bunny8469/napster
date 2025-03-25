<script>
  import { GetPeers, DownloadFile } from './lib/wailsjs/go/main/App';

  let peers = [];
  let downloadMessage = "";

  // Fetch the peer list from Go (gRPC)
  async function fetchPeers() {
    try {
      peers = await GetPeers();
    } catch (error) {
      console.error("Error fetching peers:", error);
      peers = ["Error fetching peers"];
    }
  }

  // Download a file via gRPC
  async function handleDownload(fileName) {
    try {
      downloadMessage = await DownloadFile(fileName);
    } catch (error) {
      console.error("Error downloading file:", error);
      downloadMessage = "Download failed";
    }
  }

  // Fetch peers when the component loads
  fetchPeers();
</script>

<main class="flex flex-col items-center justify-center h-screen bg-gray-900 text-white">
  <h1 class="text-4xl font-bold text-blue-500">Napster P2P Client</h1>

  <section class="mt-6 w-1/2">
    <h2 class="text-2xl font-semibold">Connected Peers</h2>
    <button class="mt-2 bg-green-500 hover:bg-green-700 text-white font-bold py-2 px-4 rounded"
      on:click={fetchPeers}>
      Refresh Peers
    </button>
    <ul class="mt-4 space-y-2 text-lg">
      {#each peers as peer}
        <li class="bg-gray-800 px-4 py-2 rounded">{peer}</li>
      {/each}
    </ul>
  </section>

  <section class="mt-8 w-1/2">
    <h2 class="text-2xl font-semibold">Download a File</h2>
    <button class="mt-2 bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded"
      on:click={() => handleDownload("test.mp4")}>
      Download test.mp4
    </button>
    {#if downloadMessage}
      <p class="mt-4 text-lg bg-gray-800 px-4 py-2 rounded">{downloadMessage}</p>
    {/if}
  </section>
</main>
