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
    setTimeout(function () {
        const rect = document.getElementById('root').getBoundingClientRect();
        window.rootLoaded(rect.left, rect.top, rect.width, rect.height);
    }, 100);
});
