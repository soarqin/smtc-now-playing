// Format time in MM:SS format
function formatTime(seconds) {
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = Math.floor(seconds % 60);
    return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`;
}

// Check if text overflows and add scroll animation if needed
function checkTextOverflow(element) {
    if (element.scrollWidth > element.clientWidth) {
        const scrollDistance = element.scrollWidth - element.clientWidth;
        element.style.setProperty('--scroll-distance', `-${scrollDistance}px`);
        element.classList.remove('center');
        element.classList.add('scroll');
    } else {
        element.classList.remove('scroll');
        element.classList.add('center');
        element.style.removeProperty('--scroll-distance');
    }
}

// Update the UI with track information
function updateTrackInfo(track) {
    const trackName = document.getElementById('trackName');
    const artistName = document.getElementById('artistName');
    if (trackName) {
        trackName.textContent = track.title ? ((artistName || !track.artist) ? track.title : (track.artist + ' - ' + track.title)) : '未播放';
        // Check for overflow after updating text
        setTimeout(() => checkTextOverflow(trackName), 0);
    }
    if (artistName) {
        artistName.textContent = track.artist || '未知艺术家';
        // Check for overflow after updating text
        setTimeout(() => checkTextOverflow(artistName), 0);
    }
    const albumArtImg = document.getElementById('albumArt');
    if (!albumArtImg) {
        return
    }
    const albumArtDiv = albumArtImg.parentElement;
    if (track.albumArt && track.albumArt.trim() !== '') {
        albumArtImg.src = track.albumArt + '?v=' + Date.now();
        albumArtDiv.style.display = 'block';
    } else {
        albumArtImg.src = 'data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs%3D';
        albumArtDiv.style.display = 'none';
    }
}

// Update progress bar and time
function updateProgress(currentTime, duration, status) {
    const progressBar = document.getElementById('progress');
    const currentTimeText = document.getElementById('currentTime');
    const totalTimeText = document.getElementById('totalTime');
    if (duration > 0) {
        const progress = (currentTime / duration) * 100;
        if (progressBar) {
            progressBar.style.width = `${progress}%`;
        }
        if (currentTimeText) {
            currentTimeText.textContent = formatTime(currentTime);
        }
        if (totalTimeText) {
            totalTimeText.textContent = formatTime(duration);
        }
    } else {
        if (progressBar) {
            progressBar.style.width = '0';
        }
        if (currentTimeText) {
            currentTimeText.textContent = '';
        }
        if (totalTimeText) {
            totalTimeText.textContent = '';
        }
    }
    const statusText = document.getElementById('status');
    if (!statusText) {
        return
    }
    statusText.textContent = (() => {
        switch (status) {
            case 0:
            case 1:
            case 2:
            case 3:
                return '已停止';
            case 4:
                return '正在播放';
            case 5:
                return '已暂停';
            default:
                return '未知状态';
        }
    })();
}

document.addEventListener('DOMContentLoaded', function() {
    setTimeout(() => {
        const infoChangedEvt = new EventSource("/info_changed")
        const progressChangedEvt = new EventSource("/progress_changed")

        infoChangedEvt.onmessage = function (event) {
            const data = JSON.parse(event.data)
            updateTrackInfo(data)
        }
        
        infoChangedEvt.onerror = function (event) {
            console.error("EventSource failed:", event)
            infoChangedEvt.close()
            setTimeout(() => {
                infoChangedEvt = new EventSource("/info_changed")
            }, 1000)
        }
        
        progressChangedEvt.onmessage = function (event) {
            const data = JSON.parse(event.data)
            updateProgress(data.position, data.duration, data.status)
        }
        
        progressChangedEvt.onerror = function (event) {
            console.error("EventSource failed:", event)
            progressChangedEvt.close()
            setTimeout(() => {
                progressChangedEvt = new EventSource("/progress_changed")
            }, 1000)
        }
    }, 100);
});
