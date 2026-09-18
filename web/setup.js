const form = document.querySelector('#settings-form');
const plexURL = document.querySelector('#plex-url');
const plexToken = document.querySelector('#plex-token');
const plexUser = document.querySelector('#plex-user');
const pollInterval = document.querySelector('#poll-interval');
const allowInsecureHTTP = document.querySelector('#allow-insecure-http');
const message = document.querySelector('#message');
const tokenHint = document.querySelector('#token-hint');
const saveButton = document.querySelector('#save');
const testButton = document.querySelector('#test');
const stopButton = document.querySelector('#stop-server');
let controlToken = '';

saveButton.disabled = true;
testButton.disabled = true;
stopButton.disabled = true;

function showMessage(text, kind = '') {
  message.textContent = text;
  message.className = `message ${kind}`;
}

async function loadSettings() {
  try {
    const [tokenResponse, settingsResponse] = await Promise.all([
      fetch('/api/control-token', { cache: 'no-store' }),
      fetch('/api/settings')
    ]);
    if (!tokenResponse.ok || !settingsResponse.ok) throw new Error('Control authorization is unavailable');
    controlToken = (await tokenResponse.json()).controlToken || '';
    if (!controlToken) throw new Error('Control authorization is unavailable');
    const settings = await settingsResponse.json();
    plexURL.value = settings.plexUrl || '';
    plexUser.value = settings.plexUser || '';
    pollInterval.value = String(settings.pollIntervalSeconds || 3);
    allowInsecureHTTP.checked = Boolean(settings.allowInsecureHttp);
    if (settings.tokenConfigured) {
      plexToken.placeholder = 'Saved token (leave blank to keep it)';
      tokenHint.firstChild.textContent = 'A token is saved. Enter a new one only to replace it. ';
    }
    document.querySelector('#app-version').textContent = settings.version || '';
    document.querySelector('#save-location').textContent = `Settings: ${settings.configPath}`;
    saveButton.disabled = false;
    testButton.disabled = false;
    stopButton.disabled = false;
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
      headers: { 'Content-Type': 'application/json', 'X-Plex-Overlay-Control-Token': controlToken },
      body: JSON.stringify({
        plexUrl: plexURL.value,
        plexToken: plexToken.value,
        plexUser: plexUser.value,
        pollIntervalSeconds: Number(pollInterval.value),
        allowInsecureHttp: allowInsecureHTTP.checked
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
    const response = await fetch('/api/test-connection', {
      method: 'POST',
      headers: { 'X-Plex-Overlay-Control-Token': controlToken }
    });
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

stopButton.addEventListener('click', async () => {
  if (!window.confirm('Stop Plex Song OBS Overlay? Your saved settings will not be removed.')) return;
  await fetch('/api/shutdown', {
    method: 'POST',
    headers: { 'X-Plex-Overlay-Control-Token': controlToken }
  });
  document.body.innerHTML = '<main class="shell"><section class="panel"><h1>Application stopped</h1><p class="intro">This window will close. Double-click the application to start it again.</p></section></main>';
});

loadSettings();
