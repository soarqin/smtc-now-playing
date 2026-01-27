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
    // Determine WebSocket URL based on current protocol
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = protocol + '//' + window.location.host + '/ws'
    const infoChangedEvt = addWebSocket(wsUrl, function (event) {
        const data = JSON.parse(event.data)
        switch (data.type) {
            case 'info':
                window.setTrackInfo(data.data.title, data.data.artist)
                window.setAlbumArt(data.data.albumArt)
                break
            case 'progress':
                window.setProgress(data.data.position, data.data.duration)
                var status = data.data.status;
                if (status !== undefined)
                    window.setPlayingStatus(data.data.status)
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
            window.rootLoaded(rect.left, rect.top, width, height);
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
