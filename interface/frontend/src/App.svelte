<script>
  import "./app.css";
  import Header from "$lib/components/ui/Header.svelte";
  import Search from "$lib/components/music/Search.svelte";
  import Downloads from "$lib/components/music/Downloads.svelte";
  import Player from "$lib/components/music/Player.svelte";
  import Library from "$lib/components/music/Library.svelte";
  import {
    SearchSongs,
    SelectFileAndUpload,
    GetTorrents,
    GetHttpPort,
    GetLibraryTorrents,
    StopSeeding,
    EnableSeeding,
  } from "$lib/wailsjs/go/main/App";
  import { onMount } from "svelte";

  let searchQuery = "";
  let currentSong = {
    name: "Napster",
    artist: "Jahnavi, Kriti, Praneeth",
    genres: ["Genre #1", "Genre #1"],
    currentTime: "0:00",
    duration: "4:30",
    isPlaying: false,
    progress: 30, // Progress as percentage
    size: "100 MB",
  };

  let searchResults = [];

  let torrents = [];
  let libtorrents = [];
  onMount(() => {
    async function loadInitialData() {
      try {
        torrents = await GetTorrents();
        console.log("Torrents:", torrents);
        libtorrents = await GetLibraryTorrents();
        console.log("Library Torrents:", libtorrents);
      } catch (err) {
        alert("Failed to load torrents: " + (err.message || err));
      }
    }

    loadInitialData();

    const intervalId = setInterval(async () => {
      try {
        const updatedLibtorrents = await GetLibraryTorrents();
        libtorrents = updatedLibtorrents;
        console.log("Updated Library Torrents:", updatedLibtorrents);
      } catch (err) {
        console.error("Error updating library torrents: ", err);
      }
    }, 3000); // 3 seconds

    return () => {
      clearInterval(intervalId); // clean up on component destroy
    };
  });

  function handleKeydown(e) {
    if (e.code === "Space" && e.target === document.body) {
      e.preventDefault(); // prevent page scroll
      togglePlayPause();
    }
  }

  async function handleSearch() {
    try {
      const results = await SearchSongs(searchQuery);
      // Map Go response to frontend format
      searchResults = results.map((song, idx) => ({
        id: idx + 1,
        name: song.file_name,
        artist: song.artist_name,
        size: "Unknown",
        peers: song.peer_addresses.length
      }));
    } catch (err) {
      alert("Search failed: " + (err.message || err));
    }
  }

  let audioSrc = "";
  let httpPort;

  onMount(async () => {
    try {
      httpPort = await GetHttpPort();
      console.log("Audio source:", httpPort); // Log the audio source to verify
      window.addEventListener("keydown", handleKeydown);
    } catch (err) {
      alert("Failed to get audio source: " + (err.message || err));
    }
  });

  onMount(() => {
    const audioPlayer = document.getElementById("audioPlayer");

    if (audioPlayer) {
      // Sync progress bar and current time
      audioPlayer.addEventListener("timeupdate", () => {
        const current = audioPlayer.currentTime;
        const duration = audioPlayer.duration || 1;
        currentSong.currentTime = formatTime(current);
        currentSong.progress = (current / duration) * 100;
      });

      // When audio ends, reset state
      audioPlayer.addEventListener("ended", () => {
        currentSong.isPlaying = false;
        currentSong.progress = 0;
        currentSong.currentTime = "0:00";
      });
    }
  });

  async function playMusic(songName) {
    const audioPlayer = document.getElementById("audioPlayer");

    if (!audioPlayer) return;

    // Always update the source first
    const newSrc =
      `http://localhost${httpPort}/audio/` + encodeURIComponent(songName);

    // If the current src is different, load the new source
    if (audioPlayer.src !== newSrc) {
      audioPlayer.src = newSrc;
      audioSrc = newSrc;

      // Wait for audio to be ready before playing
      audioPlayer.oncanplay = () => {
        audioPlayer
          .play()
          .catch((err) => console.error("Playback failed:", err));
        audioPlayer.oncanplay = null; // Remove the handler after it's used
      };
    } else {
      // If same song, just play
      audioPlayer.play().catch((err) => console.error("Playback failed:", err));
    }
  }

  function togglePlayPause() {
    if (!currentSong.isPlaying) {
      playMusic(currentSong.name); // Play the music if it's not playing
      console.log("Playing audio");
    } else {
      const audioPlayer = document.getElementById("audioPlayer");
      if (audioPlayer) {
        audioPlayer.pause(); // Pause if it's playing
        console.log("Paused audio");
      }
    }
    currentSong.isPlaying = !currentSong.isPlaying;
  }

  function handleTorrentOptions(option, torrent) {
    if (option === "play") {
      currentSong = {
        name: torrent.Metadata.file_name,
        artist: torrent.Metadata.artist_name,
        genres: ["Unknown"], // or get from metadata if available
        currentTime: "0:00", // Placeholder or extract from metadata
        duration: formatDuration(torrent.Metadata.duration),
        isPlaying: true,
        progress: 0,
        size: formatFileSize(torrent.Metadata.file_size),
      };
      togglePlayPause();
      return;
    }
    if (option === "info") {
      // Display torrent info in a more user-friendly way
      const createdDate = new Date(torrent.Metadata.CreatedAt).toLocaleString();
      const infoMessage = `
      Song Information:
      
      Title: ${torrent.Metadata.file_name}
      Artist: ${torrent.Metadata.artist_name}
      File Size: ${formatFileSize(torrent.Metadata.file_size)}
      Duration: ${formatDuration(torrent.Metadata.duration)}
      Created: ${createdDate}
      Available Peers: ${torrent.Metadata.peers ? torrent.Metadata.peers.length : 0}
    `;
      alert(infoMessage);
      return;
    }
    else if (option === "toggle-seed") {
      if (torrent.Status == "Downloaded") {
        EnableSeeding(torrent.Metadata.file_name)
      }
      else {
        StopSeeding(torrent.Metadata.file_name)
      }
    }
    // alert(`Action '${option}' on torrent: ${torrent.Metadata.file_name}`);
  }

  // Helper function to format file size from bytes to human-readable format
  function formatFileSize(bytes) {
    if (!bytes) return "Unknown";

    const units = ["B", "KB", "MB", "GB"];
    let size = parseInt(bytes);
    let unitIndex = 0;

    while (size >= 1024 && unitIndex < units.length - 1) {
      size /= 1024;
      unitIndex++;
    }

    return `${size.toFixed(2)} ${units[unitIndex]}`;
  }

  // Helper function to format duration from seconds to mm:ss format
  function formatDuration(seconds) {
    if (!seconds) return "0:00";

    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, "0")}`;
  }

  function openSettings() {
    // Implementation for opening settings
    alert("Opening settings dialog");
  }

  function stopSeeding() {
    // Implementation for stopping seeding
    alert("Stopping all seeding");
  }

  function handleSliderChange(e) {
    const audioPlayer = document.getElementById("audioPlayer");
    const newProgress = e.target.value;

    if (audioPlayer && audioPlayer.duration) {
      const newTime = (newProgress / 100) * audioPlayer.duration;
      audioPlayer.currentTime = newTime;
      currentSong.progress = newProgress;
      currentSong.currentTime = formatTime(newTime);
    }
  }
  // Helper function to parse duration string like "3:45" to seconds
  function parseDuration(duration) {
    const parts = duration.split(":");
    return parseInt(parts[0]) * 60 + parseInt(parts[1]);
  }

  // Helper function to format seconds to "m:ss" format
  function formatTime(timeInSeconds) {
    const totalSeconds = Math.floor(timeInSeconds); // removes milliseconds
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = (totalSeconds % 60).toString().padStart(2, "0");
    return `${minutes}:${seconds}`;
  }

  import "./index.css";
  let fileInput;
</script>

<main class="min-h-screen bg-[#0f0f0f] text-[#e0e0e0]">
  <Header {stopSeeding} />

  <div class="grid grid-cols-1 md:grid-cols-3 gap-4 p-4">
    <div class="md:col-span-2 flex flex-col gap-4">
      <!-- <Search {searchQuery} {searchResults} {handleSearch} {downloadSong} /> -->
      <Search bind:searchQuery {searchResults} {handleSearch} />
      <Downloads bind:torrents {handleTorrentOptions} />
    </div>

    <div class="flex flex-col gap-4 sticky top-4 h-[calc(100vh-2rem)]">
      <audio id="audioPlayer" src={audioSrc} controls hidden></audio>
      <Player {currentSong} {togglePlayPause} {handleSliderChange} />
      <Library {libtorrents} />
    </div>
  </div>
</main>
