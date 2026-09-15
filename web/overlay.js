const card = document.querySelector('#card');
const artwork = document.querySelector('#artwork');
const title = document.querySelector('#title');
const artist = document.querySelector('#artist');
const album = document.querySelector('#album');
const state = document.querySelector('#state');
const progress = document.querySelector('#progress-bar');

let lastArtwork = '';

async function refresh() {
  try {
    const response = await fetch('/api/now-playing', { cache: 'no-store' });
    if (!response.ok) throw new Error(`status ${response.status}`);
    const song = await response.json();
    if (!song.playing && !song.paused) {
      card.classList.add('hidden');
      return;
    }

    title.textContent = song.title || 'Unknown title';
    artist.textContent = song.artist || 'Unknown artist';
    album.textContent = song.album || '';
    state.textContent = song.paused ? 'PAUSED' : 'NOW PLAYING';
    card.classList.toggle('paused', song.paused);
    const percent = song.durationMs > 0 ? Math.min(100, song.positionMs / song.durationMs * 100) : 0;
    progress.style.width = `${percent}%`;

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
  }
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
  setInterval(refresh, interval);
}

start();
