// Parse URL parameters and set page max width
function setElementWidthsFromGETArgs() {
    const urlParams = new URLSearchParams(window.location.search);
    const maxWidth = urlParams.get('maxWidth');
    const artWidth = urlParams.get('artWidth');
    if (maxWidth) {
        // Convert to number and validate
        const width = parseInt(maxWidth);
        if (!isNaN(width) && width > 0) {
            // Set max-width on the container
            const container = document.querySelector('.player-card');
            if (container) {
                container.style.maxWidth = width + 'px';
            }
        }
    }
    if (artWidth) {
        const width = parseInt(artWidth);
        if (!isNaN(width) && width > 0) {
            const albumArt = document.querySelector('.album-art');
            if (albumArt) {
                albumArt.style.width = width + 'px';
                albumArt.style.height = width + 'px';
            }
        }
    }
}

function addWebSocket(wsUrl, onmessage) {
    let ws = new WebSocket(wsUrl)
    ws.onopen = function () {
        console.log("WebSocket connected to " + wsUrl)
    }
    ws.onmessage = function (event) {
        onmessage(event)
    }
    ws.onerror = function (error) {
        console.error("WebSocket " + wsUrl + " error:", error)
    }
    ws.onclose = function (event) {
        console.log("WebSocket closed, reconnecting in 1 second...")
        setTimeout(() => {
            addWebSocket(wsUrl, onmessage)
        }, 1000)
    }
    return ws
}

document.addEventListener('DOMContentLoaded', function () {
    setElementWidthsFromGETArgs();
    window.onLoaded();

    // Parse hideDelay URL param: ms of grace period before idle class (default 5000)
    var params = new URLSearchParams(window.location.search);
    var hideDelay = parseInt(params.get('hideDelay') || '5000', 10);

    // Playback state for client-side position interpolation
    var playbackState = {
        position: 0,
        duration: 0,
        lastUpdatedTime: 0,
        playbackRate: 0,
        status: 0
    };
    var rafId = null;
    var idleTimer = null;

    // Current track info for song-change transition detection
    var currentTitle = null;
    var currentArtist = null;

    // Apply a single status CSS class to the root <html> element
    function setStatusClass(className) {
        document.documentElement.classList.remove('playing', 'paused', 'stopped', 'idle');
        document.documentElement.classList.add(className);
    }

    // Start requestAnimationFrame loop for smooth progress bar interpolation
    function startAnimationLoop() {
        if (rafId !== null) {
            cancelAnimationFrame(rafId);
        }
        function animationLoop() {
            if (playbackState.playbackRate > 0 && playbackState.status === 4) {
                var elapsed = (Date.now() - playbackState.lastUpdatedTime) * playbackState.playbackRate / 1000;
                var currentPos = Math.min(playbackState.position + elapsed, playbackState.duration);
                if (typeof window.setProgress === 'function') {
                    window.setProgress(currentPos, playbackState.duration);
                }
            }
            rafId = requestAnimationFrame(animationLoop);
        }
        rafId = requestAnimationFrame(animationLoop);
    }

    // Stop the requestAnimationFrame loop
    function stopAnimationLoop() {
        if (rafId !== null) {
            cancelAnimationFrame(rafId);
            rafId = null;
        }
    }

    // Determine WebSocket URL based on current protocol
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = protocol + '//' + window.location.host + '/ws'
    addWebSocket(wsUrl, function (event) {
        const data = JSON.parse(event.data)
        switch (data.type) {
            case 'info': {
                var title = data.data.title;
                var artist = data.data.artist;
                var albumArt = data.data.albumArt;

                var isFirstTrack = (currentTitle === null && currentArtist === null);
                var isSameTrack = (title === currentTitle && artist === currentArtist);

                if (isFirstTrack || isSameTrack) {
                    // First track or same track: update immediately without transition
                    window.setTrackInfo(title, artist);
                    window.setAlbumArt(albumArt);
                } else {
                    // Song changed: add transitioning class, delay DOM update for CSS fade-out
                    document.documentElement.classList.add('transitioning');
                    // Capture values for the closure
                    var pendingTitle = title;
                    var pendingArtist = artist;
                    var pendingAlbumArt = albumArt;
                    setTimeout(function () {
                        window.setTrackInfo(pendingTitle, pendingArtist);
                        window.setAlbumArt(pendingAlbumArt);
                        document.documentElement.classList.remove('transitioning');
                    }, 300);
                }

                // Update current track state immediately
                currentTitle = title;
                currentArtist = artist;

                // Call enriched info callback if available
                if (typeof window.setExtendedInfo === 'function') {
                    window.setExtendedInfo({
                        albumTitle: data.data.albumTitle || '',
                        albumArtist: data.data.albumArtist || '',
                        playbackType: data.data.playbackType || 0,
                        sourceApp: data.data.sourceApp || ''
                    });
                }
                break;
            }
            case 'progress': {
                var pos = data.data.position;
                var dur = data.data.duration;
                var status = data.data.status;
                var lastUpdatedTime = data.data.lastUpdatedTime || 0;
                var playbackRate = data.data.playbackRate || 0;

                // Update interpolation state
                playbackState.position = pos;
                playbackState.duration = dur;
                playbackState.lastUpdatedTime = lastUpdatedTime;
                playbackState.playbackRate = playbackRate;

                if (status !== undefined) {
                    playbackState.status = status;
                    window.setPlayingStatus(status);

                    // Update status CSS class and manage idle grace-period timer
                    if (status === 4) {
                        // Playing: cancel idle timer, apply playing class
                        if (idleTimer !== null) {
                            clearTimeout(idleTimer);
                            idleTimer = null;
                        }
                        setStatusClass('playing');
                    } else if (status === 5) {
                        // Paused: cancel idle timer, apply paused class
                        if (idleTimer !== null) {
                            clearTimeout(idleTimer);
                            idleTimer = null;
                        }
                        setStatusClass('paused');
                    } else {
                        // Stopped/Closed/Opened/Changing (0-3): apply stopped,
                        // then schedule idle class after grace period
                        setStatusClass('stopped');
                        if (idleTimer !== null) {
                            clearTimeout(idleTimer);
                        }
                        idleTimer = setTimeout(function () {
                            setStatusClass('idle');
                            idleTimer = null;
                        }, hideDelay);
                    }
                }

                // Use rAF interpolation when playing with a valid server timestamp;
                // otherwise call setProgress directly (fallback for old servers or non-playing states)
                if (status === 4 && lastUpdatedTime > 0) {
                    startAnimationLoop();
                } else {
                    stopAnimationLoop();
                    window.setProgress(pos, dur);
                }

                // Call enriched progress callback if available
                if (typeof window.setExtendedProgress === 'function') {
                    window.setExtendedProgress({
                        playbackRate: data.data.playbackRate || 1.0,
                        isShuffleActive: data.data.isShuffleActive,
                        autoRepeatMode: data.data.autoRepeatMode || 0,
                        lastUpdatedTime: data.data.lastUpdatedTime || 0
                    });
                }
                break;
            }
            case 'reload':
                location.reload()
                break
            default:
                break
        }
    });
    // Fix getBoundingClientRect() to get correct root panel size
    // Use a more reliable method that waits for all content to be rendered
    function getRootSize() {
        const root = document.getElementById('root');
        if (!root || !window.rootLoaded) {
            setTimeout(getRootSize, 50);
            return;
        }
        
        // Force reflow to ensure accurate measurements
        void root.offsetHeight;
        
        // Use multiple methods to get accurate size
        const rect = root.getBoundingClientRect();
        const offsetWidth = root.offsetWidth;
        const offsetHeight = root.offsetHeight;
        const scrollWidth = root.scrollWidth;
        const scrollHeight = root.scrollHeight;
        
        // Use offsetWidth/offsetHeight as they're more reliable for layout calculations
        // Fallback to scroll dimensions, then getBoundingClientRect
        let width = offsetWidth > 0 ? offsetWidth : (scrollWidth > 0 ? scrollWidth : rect.width);
        let height = offsetHeight > 0 ? offsetHeight : (scrollHeight > 0 ? scrollHeight : rect.height);
        
        // Ensure we have valid dimensions
        if (width > 0 && height > 0) {
            // Scale by device pixel ratio for correct DPI handling
            const dpr = window.devicePixelRatio || 1;
            window.rootLoaded(rect.left, rect.top, Math.round(width * dpr), Math.round(height * dpr));
        } else {
            // Retry if dimensions are not ready yet
            setTimeout(getRootSize, 50);
        }
    }
   
    // Wait for images to load, then get size
    function waitForImagesAndGetSize() {
        const images = document.querySelectorAll('img');
        let imagesLoaded = 0;
        const totalImages = images.length;
        
        if (totalImages === 0) {
            // No images, get size after ensuring layout is complete
            requestAnimationFrame(() => {
                requestAnimationFrame(() => {
                    setTimeout(getRootSize, 50);
                });
            });
        } else {
            // Wait for all images to load
            let allLoaded = false;
            const checkComplete = () => {
                if (allLoaded) return;
                if (imagesLoaded === totalImages) {
                    allLoaded = true;
                    requestAnimationFrame(() => {
                        requestAnimationFrame(() => {
                            setTimeout(getRootSize, 50);
                        });
                    });
                }
            };
            
            images.forEach(img => {
                if (img.complete) {
                    imagesLoaded++;
                    checkComplete();
                } else {
                    img.addEventListener('load', () => {
                        imagesLoaded++;
                        checkComplete();
                    }, { once: true });
                    img.addEventListener('error', () => {
                        imagesLoaded++;
                        checkComplete();
                    }, { once: true });
                }
            });
            
            // Fallback timeout in case images take too long
            setTimeout(() => {
                if (!allLoaded) {
                    allLoaded = true;
                    getRootSize();
                }
            }, 1000);
            
            // If all images are already loaded
            checkComplete();
        }
    }
    
    // Start the process
    waitForImagesAndGetSize();
});
