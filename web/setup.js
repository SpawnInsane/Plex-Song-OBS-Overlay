const form = document.querySelector('#settings-form');
const plexURL = document.querySelector('#plex-url');
const plexToken = document.querySelector('#plex-token');
const plexUser = document.querySelector('#plex-user');
const pollInterval = document.querySelector('#poll-interval');
const message = document.querySelector('#message');
const tokenHint = document.querySelector('#token-hint');
const saveButton = document.querySelector('#save');
const testButton = document.querySelector('#test');

function showMessage(text, kind = '') {
  message.textContent = text;
  message.className = `message ${kind}`;
}

async function loadSettings() {
  try {
    const response = await fetch('/api/settings');
    const settings = await response.json();
    plexURL.value = settings.plexUrl || '';
    plexUser.value = settings.plexUser || '';
    pollInterval.value = String(settings.pollIntervalSeconds || 3);
    if (settings.tokenConfigured) {
      plexToken.placeholder = 'Saved token (leave blank to keep it)';
      tokenHint.firstChild.textContent = 'A token is saved. Enter a new one only to replace it. ';
    }
    document.querySelector('#app-version').textContent = settings.version || '';
    document.querySelector('#save-location').textContent = `Settings: ${settings.configPath}`;
  } catch (error) {
    showMessage(`Could not load settings: ${error.message}`, 'error');
  }
}

form.addEventListener('submit', async (event) => {
  event.preventDefault();
  saveButton.disabled = true;
  showMessage('Saving…');
  try {
    const response = await fetch('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        plexUrl: plexURL.value,
        plexToken: plexToken.value,
        plexUser: plexUser.value,
        pollIntervalSeconds: Number(pollInterval.value)
      })
    });
    const result = await response.json();
    if (!response.ok) throw new Error(result.error || 'Unable to save settings');
    plexToken.value = '';
    plexToken.placeholder = 'Saved token (leave blank to keep it)';
    showMessage('Settings saved. They will still be here next time.', 'success');
  } catch (error) {
    showMessage(error.message, 'error');
  } finally {
    saveButton.disabled = false;
  }
});

testButton.addEventListener('click', async () => {
  testButton.disabled = true;
  showMessage('Contacting Plex…');
  try {
    const response = await fetch('/api/test-connection', { method: 'POST' });
    const result = await response.json();
    if (!response.ok) throw new Error(result.error || 'Connection failed');
    showMessage('Connected to Plex successfully.', 'success');
  } catch (error) {
    showMessage(error.message, 'error');
  } finally {
    testButton.disabled = false;
  }
});

document.querySelector('#show-token').addEventListener('click', (event) => {
  const showing = plexToken.type === 'text';
  plexToken.type = showing ? 'password' : 'text';
  event.currentTarget.textContent = showing ? 'Show' : 'Hide';
});

document.querySelector('#copy-url').addEventListener('click', async () => {
  await navigator.clipboard.writeText(document.querySelector('#overlay-url').textContent);
  showMessage('Overlay URL copied.', 'success');
});

document.querySelector('#stop-server').addEventListener('click', async () => {
  if (!window.confirm('Stop Plex Song OBS Overlay? Your saved settings will not be removed.')) return;
  await fetch('/api/shutdown', { method: 'POST' });
  document.body.innerHTML = '<main class="shell"><section class="panel"><h1>Application stopped</h1><p class="intro">You can close this tab. Double-click the application to start it again.</p></section></main>';
});

loadSettings();
