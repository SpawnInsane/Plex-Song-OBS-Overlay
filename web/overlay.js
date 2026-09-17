const card = document.querySelector('#card');
const artwork = document.querySelector('#artwork');
const title = document.querySelector('#title');
const artist = document.querySelector('#artist');
const album = document.querySelector('#album');
const state = document.querySelector('#state');
const progress = document.querySelector('#progress-bar');

let lastArtwork = '';
let currentTrackID = null;
let anchorPositionMs = 0;
let anchorDurationMs = 0;
let anchorTime = performance.now();
let progressPlaying = false;

function trackIdentity(song) {
  return song.trackId || [song.title, song.artist, song.album, song.durationMs, song.artworkUrl].join('\u001f');
}

function estimatedPosition(now = performance.now()) {
  const elapsed = progressPlaying ? now - anchorTime : 0;
  return Math.max(0, Math.min(anchorDurationMs, anchorPositionMs + elapsed));
}

function resetProgress() {
  currentTrackID = null;
  anchorPositionMs = 0;
  anchorDurationMs = 0;
  anchorTime = performance.now();
  progressPlaying = false;
  progress.style.width = '0%';
}

function synchronizeProgress(song) {
  const now = performance.now();
  const nextTrackID = trackIdentity(song);
  const reportedPosition = Math.max(0, Math.min(song.durationMs || 0, song.positionMs || 0));
  const estimated = estimatedPosition(now);
  const trackChanged = currentTrackID !== null && currentTrackID !== nextTrackID;
  const playbackChanged = progressPlaying !== Boolean(song.playing);
  const significantSeek = Math.abs(reportedPosition - estimated) > 1500;

  if (currentTrackID === null || trackChanged || playbackChanged || significantSeek) {
    anchorPositionMs = reportedPosition;
  } else {
    anchorPositionMs = estimated + (reportedPosition - estimated) * 0.25;
  }
  currentTrackID = nextTrackID;
  anchorDurationMs = Math.max(0, song.durationMs || 0);
  anchorTime = now;
  progressPlaying = Boolean(song.playing);
}

function animateProgress(now) {
  const percent = anchorDurationMs > 0 ? estimatedPosition(now) / anchorDurationMs * 100 : 0;
  progress.style.width = `${percent}%`;
  requestAnimationFrame(animateProgress);
}

async function refresh() {
  try {
    const response = await fetch('/api/now-playing', { cache: 'no-store' });
    if (!response.ok) throw new Error(`status ${response.status}`);
    const song = await response.json();
    if (!song.playing && !song.paused) {
      card.classList.add('hidden');
      resetProgress();
      return;
    }

    title.textContent = song.title || 'Unknown title';
    artist.textContent = song.artist || 'Unknown artist';
    album.textContent = song.album || '';
    state.textContent = song.paused ? 'PAUSED' : 'NOW PLAYING';
    card.classList.toggle('paused', song.paused);
    synchronizeProgress(song);

    if (song.artworkUrl && song.artworkUrl !== lastArtwork) {
      artwork.src = song.artworkUrl;
      artwork.hidden = false;
      lastArtwork = song.artworkUrl;
    } else if (!song.artworkUrl) {
      artwork.hidden = true;
      lastArtwork = '';
    }
    card.classList.remove('hidden');
  } catch (error) {
    console.warn('Unable to update Plex status:', error);
    card.classList.add('hidden');
    resetProgress();
  }
}

function scheduleRefresh(interval) {
  setTimeout(async () => {
    await refresh();
    scheduleRefresh(interval);
  }, interval);
}

async function start() {
  let interval = 3000;
  try {
    const response = await fetch('/api/config', { cache: 'no-store' });
    if (response.ok) {
      const config = await response.json();
      interval = config.pollIntervalMs || interval;
    }
  } catch (error) {
    console.warn('Using the default refresh interval:', error);
  }

  await refresh();
  scheduleRefresh(interval);
  requestAnimationFrame(animateProgress);
}

start();
