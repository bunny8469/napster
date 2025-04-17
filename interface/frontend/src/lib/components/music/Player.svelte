<script>
    import { Button } from "$lib/components/ui/button";
    import { SkipBack, SkipForward, Play, Pause, Shuffle, Repeat } from "lucide-svelte";

    export let currentSong;
    export let togglePlayPause;
    export let handleSliderChange;

  </script>

<div class="bg-[#121212] border border-[#2a2a2a] rounded-lg p-4 md:p-6 h-min">
    <div class="flex flex-col sm:flex-row items-start gap-4 sm:gap-6 mb-6">
      <!-- Album art - Centered on mobile -->
      <div class="mx-auto sm:mx-0 min-w-[100px] sm:w-[120px] h-[100px] sm:h-[120px] bg-gradient-to-br from-[#1a1a1a] to-[#2a2a2a] rounded-lg shadow-lg flex items-center justify-center">
        <div class="text-3xl sm:text-4xl bg-gradient-to-br from-[#2c5aa0] to-[#4a86e8] bg-clip-text text-transparent">♪</div>
      </div>
      
      <!-- Song details and controls -->
      <div class="controls-container flex-grow w-full sm:w-auto">
        <h2 class="text-lg sm:text-xl font-bold text-white truncate text-center sm:text-left">{currentSong.name}</h2>
        <p class="text-sm sm:text-md text-[#b3b3b3] mt-1 sm:mt-2 text-center sm:text-left">{currentSong.artist}</p>
        
        <!-- Controls - Secondary controls hidden when space is limited -->
        <div class="flex justify-center sm:justify-start items-center gap-2 sm:gap-4 mt-3 sm:mt-4">
          <!-- Shuffle button - hidden on small screens -->
          <div class="shuffle-btn">
            <Button variant="ghost" size="icon" class="text-[#b3b3b3] hover:text-[#4a86e8] hover:bg-[#1a1a1a] rounded-full">
              <Shuffle class="h-4 w-4 sm:h-5 sm:w-5" />
            </Button>
          </div>
          
          <!-- Essential controls (always visible) -->
          <Button variant="ghost" size="icon" class="text-[#b3b3b3] hover:text-[#4a86e8] hover:bg-[#1a1a1a] rounded-full">
            <SkipBack class="h-4 w-4 sm:h-5 sm:w-5" />
          </Button>
          <Button 
            variant="outline" 
            size="icon" 
            class="flex-shrink-0 rounded-full h-8 w-8 sm:h-10 sm:w-10 flex items-center justify-center border-[#4a86e8] bg-[#4a86e8] text-white hover:bg-[#6a9ae8] hover:border-[#6a9ae8] shadow-md"
            on:click={togglePlayPause}
          >
            {#if currentSong.isPlaying}
              <Pause class="h-3 w-3 sm:h-5 sm:w-5" />
            {:else}
              <Play class="h-3 w-3 sm:h-5 sm:w-5 ml-0.5" />
            {/if}
          </Button>
          <Button variant="ghost" size="icon" class="text-[#b3b3b3] hover:text-[#4a86e8] hover:bg-[#1a1a1a] rounded-full">
            <SkipForward class="h-4 w-4 sm:h-5 sm:w-5" />
          </Button>
          
          <!-- Repeat button - hidden on small screens -->
          <div class="repeat-btn">
            <Button variant="ghost" size="icon" class="text-[#b3b3b3] hover:text-[#4a86e8] hover:bg-[#1a1a1a] rounded-full">
              <Repeat class="h-4 w-4 sm:h-5 sm:w-5" />
            </Button>
          </div>
        </div>
      </div>
    </div>
    
    <!-- Progress slider -->
    <div class="mt-4">
      <div class="relative flex items-center group h-2">
        <!-- Background track -->
        <div class="absolute w-full h-full bg-[#4a4a4a] rounded-full"></div>
    
        <!-- Progress indicator -->
        <div
          class="absolute h-full bg-[#4a86e8] rounded-full pointer-events-none"
          style="width: {currentSong.progress}%"
        ></div>
    
        <!-- Actual input slider -->
        <input
          type="range"
          min="0"
          max="100"
          bind:value={currentSong.progress}
          on:input={handleSliderChange}
          class="
            relative w-full h-full opacity-0 cursor-pointer
            [&::-webkit-slider-thumb]:appearance-none
            [&::-webkit-slider-thumb]:h-3 [&::-webkit-slider-thumb]:w-3
            [&::-webkit-slider-thumb]:rounded-full [&::-webkit-slider-thumb]:bg-white
            [&::-webkit-slider-thumb]:opacity-0
            group-hover:[&::-webkit-slider-thumb]:opacity-100
          "
        />
      </div>
      <div class="flex justify-between text-xs mt-2 text-[#b3b3b3]">
        <span>{currentSong.currentTime}</span>
        <span>{(currentSong.duration)}</span>
      </div>
    </div>
  </div>