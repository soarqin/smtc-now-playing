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

function addEventSource(eventSourceName, onmessage) {
    const eventSource = new EventSource(eventSourceName)
    eventSource.onmessage = onmessage
    eventSource.onerror = function (event) {
        console.error("EventSource " + eventSourceName + " failed:", event)
        eventSource.close()
        setTimeout(() => {
            eventSource = addEventSource(eventSourceName, onmessage)
        }, 1000)
    }
    return eventSource
}

document.addEventListener('DOMContentLoaded', function() {
    setElementWidthsFromGETArgs();
    window.onLoaded();
    setTimeout(() => {
        const infoChangedEvt = addEventSource("/update_event", function (event) {
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
        })
    }, 100);
});
