<script>
  import "./app.css";
  import Header from '$lib/components/ui/Header.svelte';
  import Search from '$lib/components/music/Search.svelte';
  import Downloads from '$lib/components/music/Downloads.svelte';
  import Player from '$lib/components/music/Player.svelte';
  import Library from '$lib/components/music/Library.svelte';
    
  let searchQuery = "";
  let currentSong = {
    name: "Song Name",
    artist: "Artist Name",
    genres: ["Genre #1", "Genre #1"],
    currentTime: "1:17",
    duration: "3:45",
    isPlaying: false,
    progress: 30 // Progress as percentage
  };
  
  let searchResults = [
    { id: 1, name: "Feel Good Inc", artist: "Gorillaz", size: "4.5 MB", hasVideo: true },
    { id: 2, name: "Hotel California", artist: "Eagles", size: "6.2 MB", hasVideo: false },
    { id: 3, name: "Sweet Child O' Mine", artist: "Guns N' Roses", size: "5.8 MB", hasVideo: true },
    { id: 4, name: "Billie Jean", artist: "Michael Jackson", size: "4.2 MB", hasVideo: true },
    { id: 5, name: "Smells Like Teen Spirit", artist: "Nirvana", size: "5.1 MB", hasVideo: true },
    { id: 6, name: "Imagine", artist: "John Lennon", size: "3.8 MB", hasVideo: false },
  ];
  
  let torrents = [
    { 
      id: 1, 
      name: "Harleys In Hawaii", 
      artist: "Katy Perry", 
      hasVideo: true,
      peers: 35, 
      progress: 100, 
      status: "Complete", 
      size: "8.2 MB", 
      speed: "0 KB/s" 
    },
    { 
      id: 2, 
      name: "Blood (From \"Marco\")", 
      artist: "Ravi Basrur, Dabzee, Rohith Sj", 
      hasVideo: true,
      peers: 42, 
      progress: 78, 
      status: "Downloading", 
      size: "12.4 MB", 
      speed: "1.2 MB/s" 
    },
    { 
      id: 3, 
      name: "Poker Face", 
      artist: "Lady Gaga", 
      hasVideo: false,
      peers: 89, 
      progress: 64, 
      status: "Downloading", 
      size: "5.7 MB", 
      speed: "856 KB/s" 
    },
    { 
      id: 4, 
      name: "Tony's Mayhem", 
      artist: "Ravi Basrur", 
      hasVideo: false,
      peers: 12, 
      progress: 45, 
      status: "Downloading", 
      size: "6.8 MB", 
      speed: "320 KB/s" 
    },
    { 
      id: 5, 
      name: "Firework", 
      artist: "Katy Perry", 
      hasVideo: false,
      peers: 67, 
      progress: 100, 
      status: "Complete", 
      size: "7.1 MB", 
      speed: "0 KB/s" 
    },
    { 
      id: 6, 
      name: "No Time for Caution", 
      artist: "Hans Zimmer", 
      hasVideo: false,
      peers: 28, 
      progress: 100, 
      status: "Complete", 
      size: "14.2 MB", 
      speed: "0 KB/s" 
    },
    { 
      id: 7, 
      name: "Lokiverse - Background Score", 
      artist: "Anirudh Ravichander", 
      hasVideo: false,
      peers: 51, 
      progress: 32, 
      status: "Downloading", 
      size: "24.6 MB", 
      speed: "1.8 MB/s" 
    },
    { 
      id: 8, 
      name: "Thunder", 
      artist: "Imagine Dragons", 
      hasVideo: true,
      peers: 74, 
      progress: 59, 
      status: "Downloading", 
      size: "9.3 MB", 
      speed: "750 KB/s" 
    },
    { 
      id: 9, 
      name: "Bohemian Rhapsody", 
      artist: "Queen", 
      hasVideo: true,
      peers: 112, 
      progress: 100, 
      status: "Complete", 
      size: "16.7 MB", 
      speed: "0 KB/s" 
    },
    { 
      id: 10, 
      name: "Shape of You", 
      artist: "Ed Sheeran", 
      hasVideo: false,
      peers: 87, 
      progress: 100, 
      status: "Complete", 
      size: "7.8 MB", 
      speed: "0 KB/s" 
    }
  ];
  
  function handleSearch() {
    // Implementation for search functionality
    alert("Searching for: " + searchQuery);
  }
  
  function togglePlayPause() {
    currentSong.isPlaying = !currentSong.isPlaying;
  }
  
  function downloadSong(song) {
    // Implementation for downloading song from search results
    alert("Downloading song: " + song.name);
  }
  
  function handleTorrentOptions(option, torrent) {
    // Implementation for handling torrent options
    alert(`Action '${option}' on torrent: ${torrent.name}`);
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
    // Update song progress based on slider value
    currentSong.progress = e.target.value;

    // Calculate time based on percentage
    let totalSeconds = parseDuration(currentSong.duration);
    let currentSeconds = Math.floor(totalSeconds * (currentSong.progress / 100));
    currentSong.currentTime = formatTime(currentSeconds);
  }
  
  // Helper function to parse duration string like "3:45" to seconds
  function parseDuration(duration) {
    const parts = duration.split(':');
    return parseInt(parts[0]) * 60 + parseInt(parts[1]);
  }
  
  // Helper function to format seconds to "m:ss" format
  function formatTime(seconds) {
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;
    return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
  }

  import './index.css'
</script>

<main class="min-h-screen bg-[#0f0f0f] text-[#e0e0e0]">
  <Header {stopSeeding} />
  
  <div class="grid grid-cols-1 md:grid-cols-3 gap-4 p-4">
    <div class="md:col-span-2 flex flex-col gap-4">
      <Search {searchQuery} {searchResults} {handleSearch} {downloadSong} />
      <Downloads {torrents} {handleTorrentOptions} />
    </div>

    <div class="flex flex-col gap-4 sticky top-4 h-[calc(100vh-2rem)]">
      <Player {currentSong} {togglePlayPause} {handleSliderChange} />
      <Library {torrents} {handleTorrentOptions} />
    </div>
  </div>
</main>