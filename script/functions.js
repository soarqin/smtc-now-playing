// script/functions.js
//
// Client bootstrap for smtc-now-playing themes. Speaks WebSocket v2:
// every frame is an envelope {"type":X,"v":2,"ts":N,"id":"...","data":{...}}.
//
// The theme defines a set of callbacks on `window` (see README > Theme
// Development). This file wires those callbacks to the server without
// requiring themes to know anything about the wire protocol.

// Wire protocol version. Server frames with a different `v` are dropped.
const PROTOCOL_VERSION = 2;

// Status values defined by the server (mirrors internal/domain/PlaybackStatus).
const STATUS_PLAYING = 4;
const STATUS_PAUSED = 5;

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

// Exponential-backoff reconnect state. Reset to RECONNECT_MIN on successful
// open so a transient server restart doesn't leave us stuck at the ceiling.
const RECONNECT_MIN = 1000;
const RECONNECT_MAX = 30000;
let reconnectDelay = RECONNECT_MIN;

// Active socket; shared so send() can reach it without being re-created on
// every reconnect. Tests can observe state via window.__wsState.
let currentSocket = null;
window.__wsState = 'disconnected';

// Build a v2 envelope and send it over the currently-open socket. Silently
// drops if the socket is not OPEN; callers don't need to care.
function send(type, data, id) {
    if (!currentSocket || currentSocket.readyState !== WebSocket.OPEN) {
        return;
    }
    const msg = { type: type, v: PROTOCOL_VERSION, ts: Date.now() };
    if (id) msg.id = id;
    if (data !== undefined) msg.data = data;
    try {
        currentSocket.send(JSON.stringify(msg));
    } catch (e) {
        console.error('WebSocket send failed:', e);
    }
}

function addWebSocket(wsUrl, onmessage) {
    let ws = new WebSocket(wsUrl);
    currentSocket = ws;
    ws.onopen = function () {
        console.log('WebSocket connected to ' + wsUrl);
        reconnectDelay = RECONNECT_MIN;
        window.__wsState = 'connected';
    };
    ws.onmessage = function (event) {
        try {
            onmessage(event);
        } catch (e) {
            // A malformed server message or a theme bug must not break
            // the rest of the socket lifecycle; log and keep running.
            console.error('WebSocket onmessage handler threw:', e);
        }
    };
    ws.onerror = function (error) {
        console.error('WebSocket ' + wsUrl + ' error:', error);
    };
    ws.onclose = function () {
        window.__wsState = 'disconnected';
        if (currentSocket === ws) {
            currentSocket = null;
        }
        const delay = reconnectDelay;
        reconnectDelay = Math.min(reconnectDelay * 2, RECONNECT_MAX);
        console.log('WebSocket closed, reconnecting in ' + delay + 'ms...');
        setTimeout(function () {
            addWebSocket(wsUrl, onmessage);
        }, delay);
    };
    return ws;
}

document.addEventListener('DOMContentLoaded', function () {
    setElementWidthsFromGETArgs();
    // Themes are expected to define window.onLoaded, but we guard against a
    // missing definition so a broken / simplified theme file doesn't abort
    // the rest of the bootstrap code (WebSocket connection, root-size probe).
    if (typeof window.onLoaded === 'function') {
        try {
            window.onLoaded();
        } catch (e) {
            console.error('theme onLoaded threw:', e);
        }
    }

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
    // Track the last album-art URL actually pushed to the theme so we can
    // skip redundant setAlbumArt calls. Themes (notably `modern`) run a
    // visible opacity crossfade on every setAlbumArt invocation; calling
    // with an unchanged URL produces a spurious fade-out / fade-in cycle.
    var currentAlbumArt = '';
    // Pending track info during a song-change transition. Any follow-up
    // info event that arrives within the 300ms transition window (e.g.
    // a late-arriving album art) updates this in place so the timeout
    // below always applies the freshest values, never the stale ones
    // captured when the transition was first scheduled.
    var transitionTimer = null;
    var pendingTrackInfo = null;

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
            if (playbackState.playbackRate > 0 && playbackState.status === STATUS_PLAYING) {
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

    function handleInfo(data) {
        if (!data) return;
        var title = data.title;
        var artist = data.artist;
        var albumArt = data.albumArt || '';

        var isFirstTrack = (currentTitle === null && currentArtist === null);
        var isSameTrack = (title === currentTitle && artist === currentArtist);

        if (isFirstTrack || isSameTrack) {
            // First track or same track.
            if (transitionTimer !== null) {
                // A song-change transition is still pending. Overwrite
                // the pending payload with the latest values so the
                // timeout applies the freshest data (e.g. an album
                // art that arrived after the initial event). Avoid
                // touching the DOM here to not fight the fade-in.
                pendingTrackInfo = { title: title, artist: artist, albumArt: albumArt };
            } else {
                window.setTrackInfo(title, artist);
                // Dedup: only push album art when the URL actually changed.
                // Prevents a visible re-fade on every same-track info event.
                if (albumArt !== currentAlbumArt) {
                    currentAlbumArt = albumArt;
                    window.setAlbumArt(albumArt);
                }
            }
        } else {
            // Song changed: add transitioning class, delay DOM update for CSS fade-out.
            document.documentElement.classList.add('transitioning');
            // Store pending info in a closure-free variable so subsequent
            // same-track events arriving within the 300ms window update
            // it in place. Do not capture in the setTimeout's closure.
            pendingTrackInfo = { title: title, artist: artist, albumArt: albumArt };
            if (transitionTimer !== null) {
                clearTimeout(transitionTimer);
            }
            transitionTimer = setTimeout(function () {
                transitionTimer = null;
                if (pendingTrackInfo !== null) {
                    window.setTrackInfo(pendingTrackInfo.title, pendingTrackInfo.artist);
                    window.setAlbumArt(pendingTrackInfo.albumArt);
                    currentAlbumArt = pendingTrackInfo.albumArt;
                    pendingTrackInfo = null;
                }
                document.documentElement.classList.remove('transitioning');
            }, 300);
        }

        // Update current track state immediately
        currentTitle = title;
        currentArtist = artist;

        // Call enriched info callback if available
        if (typeof window.setExtendedInfo === 'function') {
            window.setExtendedInfo({
                albumTitle: data.albumTitle || '',
                albumArtist: data.albumArtist || '',
                playbackType: data.playbackType || 0,
                sourceApp: data.sourceApp || ''
            });
        }
    }

    function handleProgress(data) {
        if (!data) return;
        var pos = data.position;
        var dur = data.duration;
        var status = data.status;
        var lastUpdatedTime = data.lastUpdatedTime || 0;
        var playbackRate = data.playbackRate || 0;

        // Update interpolation state
        playbackState.position = pos;
        playbackState.duration = dur;
        playbackState.lastUpdatedTime = lastUpdatedTime;
        playbackState.playbackRate = playbackRate;

        if (status !== undefined) {
            playbackState.status = status;
            window.setPlayingStatus(status);

            // Update status CSS class and manage idle grace-period timer
            if (status === STATUS_PLAYING) {
                // Playing: cancel idle timer, apply playing class
                if (idleTimer !== null) {
                    clearTimeout(idleTimer);
                    idleTimer = null;
                }
                setStatusClass('playing');
            } else if (status === STATUS_PAUSED) {
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
        // otherwise call setProgress directly (fallback for non-playing states)
        if (status === STATUS_PLAYING && lastUpdatedTime > 0) {
            startAnimationLoop();
        } else {
            stopAnimationLoop();
            window.setProgress(pos, dur);
        }

        // Call enriched progress callback if available
        if (typeof window.setExtendedProgress === 'function') {
            window.setExtendedProgress({
                playbackRate: data.playbackRate || 1.0,
                isShuffleActive: data.isShuffleActive,
                autoRepeatMode: data.autoRepeatMode || 0,
                lastUpdatedTime: data.lastUpdatedTime || 0
            });
        }
    }

    // Determine WebSocket URL based on current protocol
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = protocol + '//' + window.location.host + '/ws';
    addWebSocket(wsUrl, function (event) {
        let env;
        try {
            env = JSON.parse(event.data);
        } catch (e) {
            console.error('malformed WebSocket message:', e, event.data);
            return;
        }
        if (!env || typeof env.type !== 'string') {
            return;
        }
        // Drop frames from an older/newer server that doesn't match our v2 wire.
        if (env.v !== PROTOCOL_VERSION) {
            console.warn('Unsupported WS protocol version:', env.v);
            return;
        }

        switch (env.type) {
            case 'hello': {
                // Server handshake: log version and capabilities for debugging.
                var hello = env.data || {};
                console.log('WS hello from server', hello.serverVersion || '', hello.capabilities || {});
                break;
            }
            case 'info':
                handleInfo(env.data);
                break;
            case 'progress':
                handleProgress(env.data);
                break;
            case 'sessions':
                // Theme doesn't need session enumeration; ignore silently.
                break;
            case 'reload':
                // Give the browser a tick to flush pending work before reloading.
                setTimeout(function () { location.reload(); }, 100);
                break;
            case 'ping':
                // Server heartbeat. Echo the server's ts exactly (not our own)
                // so the server can correlate ping<->pong and keep the
                // connection considered alive.
                if (currentSocket && currentSocket.readyState === WebSocket.OPEN) {
                    try {
                        currentSocket.send(JSON.stringify({
                            type: 'pong',
                            v: PROTOCOL_VERSION,
                            ts: env.ts
                        }));
                    } catch (e) {
                        console.error('WebSocket pong send failed:', e);
                    }
                }
                break;
            case 'ack':
                // Control ack. No pending-request table yet (themes don't send
                // control from here), just log failures for visibility.
                if (env.data && env.data.success === false) {
                    console.warn('WS control failed:', env.id, env.data.error || '');
                }
                break;
            default:
                break;
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
