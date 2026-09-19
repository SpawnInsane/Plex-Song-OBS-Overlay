const form = document.querySelector('#settings-form');
const plexURL = document.querySelector('#plex-url');
const plexToken = document.querySelector('#plex-token');
const plexUser = document.querySelector('#plex-user');
const pollInterval = document.querySelector('#poll-interval');
const allowInsecureHTTP = document.querySelector('#allow-insecure-http');
const includePrereleases = document.querySelector('#include-prereleases');
const message = document.querySelector('#message');
const tokenHint = document.querySelector('#token-hint');
const saveButton = document.querySelector('#save');
const testButton = document.querySelector('#test');
const stopButton = document.querySelector('#stop-server');
const checkUpdatesButton = document.querySelector('#check-updates');
const installUpdateButton = document.querySelector('#install-update');
const updateMessage = document.querySelector('#update-message');
const updateNotes = document.querySelector('#update-notes');
let controlToken = '';

saveButton.disabled = true;
testButton.disabled = true;
stopButton.disabled = true;
checkUpdatesButton.disabled = true;
installUpdateButton.disabled = true;

function showMessage(text, kind = '') {
  message.textContent = text;
  message.className = `message ${kind}`;
}

function showUpdateMessage(text, kind = '') {
  updateMessage.textContent = text;
  updateMessage.className = `message ${kind}`;
}

function renderUpdate(info) {
  updateNotes.hidden = true;
  updateNotes.textContent = '';
  if (!info || !info.available) {
    installUpdateButton.hidden = true;
    showUpdateMessage(`You are running the latest version (${info && info.current ? info.current : 'unknown'}).`, 'success');
    return;
  }
  const label = info.prerelease ? 'pre-release' : 'release';
  showUpdateMessage(`Version ${info.latest} (${label}) is available. You are running ${info.current}.`, 'success');
  if (info.notes) {
    updateNotes.textContent = info.notes;
    updateNotes.hidden = false;
  }
  installUpdateButton.hidden = false;
  installUpdateButton.disabled = false;
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
    includePrereleases.checked = Boolean(settings.includePrereleases);
    if (settings.tokenConfigured) {
      plexToken.placeholder = 'Saved token (leave blank to keep it)';
      tokenHint.firstChild.textContent = 'A token is saved. Enter a new one only to replace it. ';
    }
    document.querySelector('#app-version').textContent = settings.version || '';
    document.querySelector('#save-location').textContent = `Settings: ${settings.configPath}`;
    saveButton.disabled = false;
    testButton.disabled = false;
    stopButton.disabled = false;
    checkUpdatesButton.disabled = false;
    loadUpdateStatus();
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
        allowInsecureHttp: allowInsecureHTTP.checked,
        includePrereleases: includePrereleases.checked
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

async function loadUpdateStatus() {
  try {
    const response = await fetch('/api/update/status');
    if (!response.ok) return;
    const status = await response.json();
    if (!status.checked) return;
    if (status.error) {
      showUpdateMessage(`Could not check for updates: ${status.error}`, 'error');
      return;
    }
    renderUpdate(status);
  } catch {
    // A failed status read is not worth interrupting setup.
  }
}

checkUpdatesButton.addEventListener('click', async () => {
  checkUpdatesButton.disabled = true;
  installUpdateButton.hidden = true;
  showUpdateMessage('Checking GitHub for a newer version…');
  try {
    const response = await fetch('/api/update/check', {
      method: 'POST',
      headers: { 'X-Plex-Overlay-Control-Token': controlToken }
    });
    const result = await response.json();
    if (!response.ok) throw new Error(result.error || 'Update check failed');
    renderUpdate(result);
  } catch (error) {
    showUpdateMessage(error.message, 'error');
  } finally {
    checkUpdatesButton.disabled = false;
  }
});

installUpdateButton.addEventListener('click', async () => {
  if (!window.confirm('Install the update and restart Plex Song OBS Overlay? The overlay will be unavailable until it restarts.')) return;
  installUpdateButton.disabled = true;
  showUpdateMessage('Downloading and verifying the update…');
  try {
    const response = await fetch('/api/update/apply', {
      method: 'POST',
      headers: { 'X-Plex-Overlay-Control-Token': controlToken }
    });
    const result = await response.json();
    if (!response.ok) throw new Error(result.error || 'Update failed');
    document.body.innerHTML = '<main class="shell"><section class="panel"><h1>Updating</h1><p class="intro">The application is restarting on the new version. This window will close.</p></section></main>';
  } catch (error) {
    showUpdateMessage(error.message, 'error');
    installUpdateButton.disabled = false;
  }
});

loadSettings();
