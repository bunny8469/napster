<script>
  import "./app.css";
  import { GetPeers, DownloadFile } from './lib/wailsjs/go/main/App';
  import { Button } from "$lib/components/ui/button";
  
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

<main></main>